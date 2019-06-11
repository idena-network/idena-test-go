package process

import (
	"errors"
	"fmt"
	"idena-test-go/apiclient"
	"idena-test-go/client"
	"idena-test-go/common"
	"idena-test-go/log"
	"idena-test-go/node"
	"idena-test-go/scenario"
	"idena-test-go/user"
	"sync"
	"time"
)

const (
	firstRpcPort  = 9010
	firstIpfsPort = 4010
	firstPort     = 40410

	invite    = "Invite"
	candidate = "Candidate"
	newbie    = "Newbie"
	verified  = "Verified"

	periodFlipLottery      = "FlipLottery"
	periodShortSession     = "ShortSession"
	periodLongSession      = "LongSession"
	periodAfterLongSession = "AfterLongSession"
	periodNone             = "None"

	stateWaitingTimeout = 180 * time.Second
	flipsWaitingTimeout = time.Minute
	DataDir             = "dataDir"
	requestRetryDelay   = 8 * time.Second

	initialRequiredFlips = 1
)

type Process struct {
	sc                     scenario.Scenario
	workDir                string
	execCommandName        string
	users                  []*user.User
	firstUser              *user.User
	godAddress             string
	bootNode               string
	ipfsBootNode           string
	ceremonyTime           int64
	testCounter            int
	reqIdHolder            *client.ReqIdHolder
	verbosity              int
	maxNetDelay            int
	rpcHost                string
	nodeBaseConfigFileName string
	nodeBaseConfigData     []byte
	godMode                bool
	godHost                string
	apiClient              *apiclient.Client
	firstPortOffset        int
}

func NewProcess(sc scenario.Scenario, firstPortOffset int, workDir string, execCommandName string, nodeBaseConfigFileName string,
	rpcHost string, verbosity int, maxNetDelay int, godMode bool, godHost string) *Process {
	var apiClient *apiclient.Client
	if !godMode {
		apiClient = apiclient.NewClient(fmt.Sprintf("http://%s:%d/", godHost, 1111))
	}
	return &Process{
		sc:                     sc,
		workDir:                workDir,
		reqIdHolder:            client.NewReqIdHolder(),
		execCommandName:        execCommandName,
		rpcHost:                rpcHost,
		verbosity:              verbosity,
		maxNetDelay:            maxNetDelay,
		nodeBaseConfigFileName: nodeBaseConfigFileName,
		godMode:                godMode,
		godHost:                godHost,
		apiClient:              apiClient,
		firstPortOffset:        firstPortOffset,
	}
}

func (process *Process) Start() {
	defer process.destroy()
	process.init()
	for {
		process.createNewUsers()
		process.switchNodes()
		if !process.checkActiveUser() {
			process.handleError(errors.New("there are no active users"), "")
		}
		process.test()
		process.testCounter++
	}
}

func (process *Process) destroy() {
	for _, u := range process.users {
		u.Node.Destroy()
	}
}

func getNodeDataDir(index int, port int) string {
	return fmt.Sprintf("datadir-%d-%d", index, port)
}

func (process *Process) createNewUsers() {
	epoch := process.getCurrentTestEpoch()
	currentUsers := len(process.users)
	newUsers := process.sc.EpochNewUsers[epoch]
	if newUsers == 0 {
		return
	}
	excludeGodNode := epoch == 0 && process.godMode
	if excludeGodNode {
		newUsers--
	}
	if newUsers > 0 {
		process.createUsers(newUsers)
	}

	var users []*user.User
	usersToStart := process.users[currentUsers : currentUsers+newUsers]
	if excludeGodNode {
		users = process.users[currentUsers-1 : currentUsers+newUsers]
	} else {
		users = usersToStart
	}

	process.startNodes(usersToStart, node.DeleteDataDir)

	process.getNodeAddresses(users)

	process.sendInvites(users)

	process.waitForInvites(users)

	process.activateInvites(users)

	process.waitForCandidates(users)

	log.Debug("New users creation completed")
}

func (process *Process) createUsers(count int) {
	currentUsersCount := len(process.users)
	for i := 0; i < count; i++ {
		process.createUser(i + currentUsersCount)
	}
	log.Info(fmt.Sprintf("Created %v users", count))
}

func (process *Process) createUser(index int) *user.User {
	rpcPort := firstRpcPort + process.firstPortOffset + index
	n := node.NewNode(index,
		process.workDir,
		process.execCommandName,
		DataDir,
		getNodeDataDir(index, rpcPort),
		firstPort+process.firstPortOffset+index,
		false,
		process.rpcHost,
		rpcPort,
		process.bootNode,
		process.ipfsBootNode,
		firstIpfsPort+process.firstPortOffset+index,
		process.godAddress,
		process.ceremonyTime,
		process.verbosity,
		process.maxNetDelay,
		process.nodeBaseConfigData,
	)
	u := user.NewUser(client.NewClient(*n, process.reqIdHolder), n, index)
	process.users = append(process.users, u)
	log.Info(fmt.Sprintf("%v created", u.GetInfo()))
	return u
}

func (process *Process) startNodes(users []*user.User, mode node.StartMode) {
	n := len(users)
	wg := sync.WaitGroup{}
	wg.Add(n)
	for _, u := range users {
		go func(u *user.User) {
			process.startNode(u, mode)
			wg.Done()
		}(u)
	}
	wg.Wait()
	log.Info(fmt.Sprintf("Started %v nodes", n))
}

func (process *Process) getNodeAddresses(users []*user.User) {
	for _, u := range users {
		var err error
		u.Address, err = u.Client.GetCoinbaseAddr()
		process.handleError(err, fmt.Sprintf("%v unable to get node address", u.GetInfo()))
		log.Info(fmt.Sprintf("%v got coinbase address %v", u.GetInfo(), u.Address))
	}
}

func (process *Process) startNode(u *user.User, mode node.StartMode) {
	u.Start(mode)
	log.Info(fmt.Sprintf("Started node %v", u.GetInfo()))
}

func (process *Process) stopNodes(users []*user.User) {
	n := len(users)
	wg := sync.WaitGroup{}
	wg.Add(n)
	for _, u := range users {
		go func(u *user.User) {
			process.stopNode(u)
			wg.Done()
		}(u)
	}
	wg.Wait()
	log.Info(fmt.Sprintf("Stopped %v nodes", n))
}

func (process *Process) stopNode(u *user.User) {
	u.Stop()
	log.Info(fmt.Sprintf("Stopped node %v", u.GetInfo()))
}

func (process *Process) sendInvites(users []*user.User) {
	if process.godMode {
		invitesCount := 0
		for _, u := range users {
			sender := process.firstUser
			invite, err := sender.Client.SendInvite(u.Address)
			process.handleError(err, fmt.Sprintf("%v unable to send invite to %v", sender.GetInfo(), u.Address))
			log.Info(fmt.Sprintf("%s sent invite %s to %s", sender.GetInfo(), invite.Hash, u.GetInfo()))
			invitesCount++
		}
		log.Info(fmt.Sprintf("Sent %v invites", invitesCount))
		return
	}

	invitesCount := 0
	for _, u := range users {
		err := process.apiClient.CreateInvite(u.Address)
		process.handleError(err, fmt.Sprintf("%v unable to request invite", u.GetInfo()))
		invitesCount++
	}
	log.Info(fmt.Sprintf("Requested %v invites", invitesCount))
	return
}

func (process *Process) waitForInvites(users []*user.User) {
	process.waitForNodesState(users, invite)
}

func (process *Process) activateInvites(users []*user.User) {
	for _, u := range users {
		hash, err := u.Client.ActivateInvite(u.Address)
		process.handleError(err, fmt.Sprintf("%v unable to activate invite for %v", u.GetInfo(), u.Address))
		log.Info(fmt.Sprintf("%s sent invite activation %s", u.GetInfo(), hash))
	}
	log.Info("Activated invites")
}

func (process *Process) waitForCandidates(users []*user.User) {
	process.waitForNodesState(users, candidate)
}

func (process *Process) waitForNodesState(users []*user.User, state string) {
	log.Info(fmt.Sprintf("Start waiting for user states %v", state))
	wg := sync.WaitGroup{}
	wg.Add(len(users))
	targetStates := []string{state}
	for _, u := range users {
		go func(u *user.User) {
			process.waitForNodeState(u, targetStates)
			wg.Done()
		}(u)
	}
	ok := common.WaitWithTimeout(&wg, stateWaitingTimeout)
	if !ok {
		process.handleError(errors.New(fmt.Sprintf("State %v waiting timeout", state)), "")
	}
	log.Info(fmt.Sprintf("Got state %v for all users", state))
}

func (process *Process) waitForNodeState(u *user.User, states []string) {
	log.Info(fmt.Sprintf("%v start waiting for one of the states %v", u.GetInfo(), states))
	var currentState string
	for {
		identity := process.getIdentity(u)
		currentState = identity.State
		log.Debug(fmt.Sprintf("%v state %v", u.GetInfo(), currentState))
		if in(currentState, states) {
			break
		}
		time.Sleep(requestRetryDelay)
	}
	log.Info(fmt.Sprintf("%v got target state %v", u.GetInfo(), currentState))
}

func (process *Process) getIdentity(u *user.User) client.Identity {
	identity, err := u.Client.GetIdentity(u.Address)
	process.handleError(err, fmt.Sprintf("%v unable to get identity", u.GetInfo()))
	return identity
}

func in(value string, list []string) bool {
	for _, elem := range list {
		if elem == value {
			return true
		}
	}
	return false
}

func (process *Process) checkActiveUser() bool {
	for _, u := range process.users {
		if u.Active {
			return true
		}
	}
	return false
}

func (process *Process) handleError(err error, prefix string) {
	if err == nil {
		return
	}
	fullPrefix := ""
	if len(prefix) > 0 {
		fullPrefix = fmt.Sprintf("%v: ", prefix)
	}
	log.Error(fmt.Sprintf("%v%v", fullPrefix, err))
	for _, u := range process.users {
		u.Node.Destroy()
	}
	panic(err)
}

func (process *Process) switchNodes() {
	epoch := process.getCurrentTestEpoch()

	nodeIndexesToStop := process.sc.EpochNodeStops[epoch]
	if len(nodeIndexesToStop) > 0 {
		process.stopNodes(process.getUsers(nodeIndexesToStop))
	}

	nodeIndexesToStart := process.sc.EpochNodeStarts[epoch]
	if len(nodeIndexesToStart) > 0 {
		process.startNodes(process.getUsers(nodeIndexesToStart), node.DeleteNothing)
	}
}

func (process *Process) getUsers(indexes []int) []*user.User {
	var nodes []*user.User
	for _, index := range indexes {
		nodes = append(nodes, process.users[index])
	}
	return nodes
}
