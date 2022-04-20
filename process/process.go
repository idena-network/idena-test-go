package process

import (
	"encoding/hex"
	"fmt"
	"github.com/idena-network/idena-go/api"
	common2 "github.com/idena-network/idena-go/common"
	"github.com/idena-network/idena-go/common/eventbus"
	"github.com/idena-network/idena-go/crypto"
	"github.com/idena-network/idena-test-go/apiclient"
	"github.com/idena-network/idena-test-go/client"
	"github.com/idena-network/idena-test-go/common"
	"github.com/idena-network/idena-test-go/log"
	"github.com/idena-network/idena-test-go/node"
	"github.com/idena-network/idena-test-go/scenario"
	"github.com/idena-network/idena-test-go/user"
	"github.com/pkg/errors"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	invite    = "Invite"
	candidate = "Candidate"
	newbie    = "Newbie"
	verified  = "Verified"
	human     = "Human"

	periodFlipLottery  = "FlipLottery"
	periodShortSession = "ShortSession"
	periodLongSession  = "LongSession"
	periodNone         = "None"
	lowPowerProfile    = "lowpower"
	sharedProfile      = "shared"

	DataDir           = "datadir"
	requestRetryDelay = 8 * time.Second

	initialRequiredFlips = 1

	shortSessionFlipKeyDeadline = time.Second * 30
	flipsWaitingMinTimeout      = time.Minute
	flipsWaitingMaxTimeout      = time.Second * 90
)

type Process struct {
	bus                           eventbus.Bus
	sc                            scenario.Scenario
	es                            *epochState
	wg                            *sync.WaitGroup
	workDir                       string
	execCommandName               string
	users                         []user.User
	godUser                       user.User
	godAddress                    string
	ipfsBootNode                  string
	ceremonyTime                  int64
	testCounter                   int
	verbosity                     int
	maxNetDelay                   int
	rpcHost                       string
	nodeBaseConfigFileName        string
	nodeBaseConfigData            []byte
	godMode                       bool
	godHost                       string
	apiClient                     *apiclient.Client
	firstPortOffset               int
	mutex                         sync.Mutex
	nodeStartWaitingTime          time.Duration
	nodeStartPauseTime            time.Duration
	nodeStopWaitingTime           time.Duration
	firstRpcPort                  int
	firstIpfsPort                 int
	firstPort                     int
	flipsChan                     chan int
	lowPowerProfileRate           float32
	lowPowerProfileCount          int
	ceremonyIntervals             *client.CeremonyIntervals
	fastNewbie                    bool
	validationOnly                bool
	allowFailNotification         bool
	minFlipSize                   int
	maxFlipSize                   int
	decryptFlips                  bool
	randomApiKeys                 bool
	predefinedApiKeys             []string
	validationTimeoutExtraMinutes int
}

func NewProcess(sc scenario.Scenario, firstPortOffset int, workDir string, execCommandName string,
	nodeBaseConfigFileName string, rpcHost string, verbosity int, maxNetDelay int, godMode bool, godHost string,
	nodeStartWaitingTime time.Duration, nodeStartPauseTime time.Duration, nodeStopWaitingTime time.Duration,
	firstRpcPort int, firstIpfsPort int, firstPort int, flipsChanSize int, lowPowerProfileRate float32,
	fastNewbie, validationOnly, allowFailNotification bool, minFlipSize int, maxFlipSize int, decryptFlips,
	randomApiKeys bool, predefinedApiKeys []string, validationTimeoutExtraMinutes int) *Process {
	var apiClient *apiclient.Client
	if !godMode {
		apiClient = apiclient.NewClient(fmt.Sprintf("http://%s:%d/", godHost, 1111))
	}
	var flipsChan chan int
	if flipsChanSize > 0 {
		flipsChan = make(chan int, flipsChanSize)
	}
	return &Process{
		sc:                            sc,
		workDir:                       workDir,
		execCommandName:               execCommandName,
		rpcHost:                       rpcHost,
		verbosity:                     verbosity,
		maxNetDelay:                   maxNetDelay,
		nodeBaseConfigFileName:        nodeBaseConfigFileName,
		godMode:                       godMode,
		godHost:                       godHost,
		apiClient:                     apiClient,
		firstPortOffset:               firstPortOffset,
		nodeStartWaitingTime:          nodeStartWaitingTime,
		nodeStartPauseTime:            nodeStartPauseTime,
		nodeStopWaitingTime:           nodeStopWaitingTime,
		firstRpcPort:                  firstRpcPort,
		firstIpfsPort:                 firstIpfsPort,
		firstPort:                     firstPort,
		flipsChan:                     flipsChan,
		lowPowerProfileRate:           lowPowerProfileRate,
		fastNewbie:                    fastNewbie,
		validationOnly:                validationOnly,
		allowFailNotification:         allowFailNotification,
		minFlipSize:                   minFlipSize,
		maxFlipSize:                   maxFlipSize,
		decryptFlips:                  decryptFlips,
		randomApiKeys:                 randomApiKeys,
		predefinedApiKeys:             predefinedApiKeys,
		validationTimeoutExtraMinutes: validationTimeoutExtraMinutes,
	}
}

func (process *Process) Start() {
	defer process.destroy()
	process.init()
	for {
		process.createEpochNewUsers(process.sc.EpochNewUsersBeforeFlips[process.getCurrentTestIndex()])
		if !process.checkActiveUser() {
			process.handleError(errors.New("there are no active users"), "")
		}
		process.test()
		process.testCounter++
	}
}

func (process *Process) destroy() {
	for _, u := range process.users {
		if err := u.DestroyNode(); err != nil {
			log.Warn(err.Error())
		}
	}
}

func getNodeDataDir(index int, port int) string {
	return fmt.Sprintf("datadir-%d-%d", index, port)
}

func (process *Process) createEpochNewUsers(epochNewUsers []*scenario.NewUsers) {
	if len(epochNewUsers) == 0 {
		return
	}
	var candidates []user.User
	for _, epochInviterNewUsers := range epochNewUsers {
		users := process.startNewNodesAndSendInvites(epochInviterNewUsers)
		if epochInviterNewUsers.Inviter != nil {
			candidates = append(candidates, users...)
		}
	}

	if process.validationOnly {
		process.waitForNewbies(candidates)
	} else {
		process.waitForInvites(candidates)

		if process.getCurrentTestIndex() == 0 && !process.godMode {
			time.Sleep(time.Second * 5)
		}

		process.activateInvites(candidates)

		if process.fastNewbie {
			process.waitForNewbies(candidates)
		} else {
			process.waitForCandidates(candidates)
		}
	}

	log.Debug("New users creation completed")
}

func (process *Process) createEpochInviterNewUsers(epochInviterNewUsers *scenario.NewUsers) []user.User {
	if epochInviterNewUsers == nil {
		return nil
	}
	users := process.startNewNodesAndSendInvites(epochInviterNewUsers)
	var candidates []user.User
	if epochInviterNewUsers.Inviter != nil {
		candidates = users
	}

	if process.validationOnly {
		process.waitForNewbies(candidates)
	} else {
		process.waitForInvites(candidates)

		if process.getCurrentTestIndex() == 0 && !process.godMode {
			time.Sleep(time.Second * 5)
		}

		process.activateInvites(candidates)

		if process.fastNewbie {
			process.waitForNewbies(candidates)
		} else {
			process.waitForCandidates(candidates)
		}
	}

	log.Debug("New users creation completed")

	return users
}

func (process *Process) startNewNodesAndSendInvites(epochInviterNewUsers *scenario.NewUsers) []user.User {
	var users []user.User
	excludeGodNode := process.godMode && len(process.users) == 1
	newUsers := epochInviterNewUsers.Count
	if excludeGodNode {
		newUsers--
	}
	process.mutex.Lock()
	currentUsers := len(process.users)
	if newUsers > 0 {
		process.createUsers(newUsers, epochInviterNewUsers.Command, epochInviterNewUsers.SharedNode)
	}
	process.mutex.Unlock()
	usersToStart := process.users[currentUsers : currentUsers+newUsers]
	if excludeGodNode {
		users = process.users[currentUsers-1 : currentUsers+newUsers]
	} else {
		users = usersToStart
	}

	process.startNodes(usersToStart, node.DeleteDataDir)

	process.getNodeAddresses(users)

	var amount float32
	if epochInviterNewUsers.InviteAmount != nil {
		amount = *epochInviterNewUsers.InviteAmount
	} else {
		amount = process.sc.InviteAmount
	}
	if epochInviterNewUsers.Inviter != nil {
		process.sendInvites(*epochInviterNewUsers.Inviter, amount, users)
	} else if amount > 0 && process.godMode {
		const SendTx = 0x0
		for _, u := range users {
			sender := process.godUser
			to := u.GetAddress()
			hash, err := sender.SendTransaction(SendTx, &to, amount, 0, nil)
			if err != nil {
				process.handleError(err, fmt.Sprintf("%v unable to send initial coins to %v", sender.GetInfo(), u.GetInfo()))
			}
			log.Info(fmt.Sprintf("%v sent initial coins, amount: %v, hash: %v", sender.GetInfo(), amount, hash))
		}

	}

	return users
}

func (process *Process) createUsers(count int, command string, sharedNode *int) {
	currentUsersCount := len(process.users)
	for i := 0; i < count; i++ {
		process.createUser(i+currentUsersCount, command, sharedNode)
	}
	log.Info(fmt.Sprintf("Created %v users", count))
}

func generateApiKey(userIndex int, randomApiKeys bool, predefinedApiKeys []string) string {
	if userIndex < len(predefinedApiKeys) {
		return predefinedApiKeys[userIndex]
	}
	if !randomApiKeys {
		return "testApiKey" + strconv.Itoa(userIndex)
	}
	randomKey, _ := crypto.GenerateKey()
	return hex.EncodeToString(crypto.FromECDSA(randomKey)[:16])
}

func (process *Process) createUser(index int, command string, sharedNode *int) user.User {

	getRpcPort := func(index int) int {
		return process.firstRpcPort + process.firstPortOffset + index
	}

	var u user.User
	if sharedNode == nil {
		rpcPort := getRpcPort(index)
		apiKey := generateApiKey(index, process.randomApiKeys, process.predefinedApiKeys)
		profile := process.defineNewNodeProfile(false, isNodeShared(index, process.sc))
		if profile == lowPowerProfile {
			process.lowPowerProfileCount++
		}
		n := node.NewNode(index,
			process.workDir,
			process.execCommandName,
			DataDir,
			getNodeDataDir(index, rpcPort),
			process.firstPort+process.firstPortOffset+index,
			false,
			process.rpcHost,
			rpcPort,
			process.ipfsBootNode,
			process.firstIpfsPort+process.firstPortOffset+index,
			process.godAddress,
			process.ceremonyTime,
			process.verbosity,
			process.maxNetDelay,
			process.nodeBaseConfigData,
			process.nodeStartWaitingTime,
			process.nodeStopWaitingTime,
			apiKey,
			profile,
			isNodeShared(index, process.sc),
		)
		if len(command) > 0 {
			n.SetExecCommandName(command)
		}
		u = user.NewUser(nil, client.NewClient(rpcPort, index, apiKey, process.bus), n, index)
	} else {
		parentUser := process.users[*sharedNode]
		rpcPort := getRpcPort(*sharedNode)
		apiKey := generateApiKey(parentUser.GetIndex(), process.randomApiKeys, process.predefinedApiKeys)
		u = user.NewUser(parentUser, client.NewClient(rpcPort, parentUser.GetIndex(), apiKey, process.bus), nil, index)
	}
	process.users = append(process.users, u)
	log.Info(fmt.Sprintf("%v created", u.GetInfo()))
	return u
}

func (process *Process) defineNewNodeProfile(isGod, isShared bool) string {
	if isShared || isGod {
		return sharedProfile
	}
	ownNodeUsers := 0
	for _, u := range process.users {
		if !u.SharedNode() {
			ownNodeUsers++
		}
	}
	if process.lowPowerProfileRate == 0 || ownNodeUsers == 0 {
		return ""
	}
	curRate := float32(process.lowPowerProfileCount) / float32(ownNodeUsers)
	if curRate > process.lowPowerProfileRate {
		return ""
	}
	return lowPowerProfile
}

func (process *Process) startNodes(users []user.User, mode node.StartMode) {
	n := len(users)
	wg := sync.WaitGroup{}
	wg.Add(n)
	for _, u := range users {
		if process.nodeStartPauseTime > 0 {
			time.Sleep(process.nodeStartPauseTime)
		}
		go func(u user.User) {
			process.startNode(u, mode)
			wg.Done()
		}(u)
	}
	wg.Wait()
	log.Info(fmt.Sprintf("Started %v nodes", n))
}

func (process *Process) getNodeAddresses(users []user.User) {
	for _, u := range users {
		err := u.InitAddress()
		process.handleError(err, fmt.Sprintf("%v unable to get node address", u.GetInfo()))
		if process.validationOnly {
			err = u.InitPubKey()
			process.handleError(err, fmt.Sprintf("%v unable to get node pub key", u.GetInfo()))
			log.Info(fmt.Sprintf("%v got coinbase address %v and pub key %v", u.GetInfo(), u.GetAddress(), u.GetPubKey()))
		} else {
			log.Info(fmt.Sprintf("%v got coinbase address %v", u.GetInfo(), u.GetAddress()))
		}
	}
}

func (process *Process) startNode(u user.User, mode node.StartMode) {
	process.handleError(u.Start(mode), "Unable to start node")
	log.Info(fmt.Sprintf("Started node %v", u.GetInfo()))
}

func (process *Process) stopNode(u user.User) {
	process.handleError(u.Stop(), "Unable to stop node")
	log.Info(fmt.Sprintf("Stopped node %v", u.GetInfo()))
}

func (process *Process) sendInvites(inviterIndex int, inviteAmount float32, users []user.User) {
	if process.godMode {
		invitesCount := 0
		for _, u := range users {
			sender := process.users[inviterIndex]
			var recipient string
			if process.validationOnly {
				recipient = u.GetPubKey()
			} else {
				recipient = u.GetAddress()
			}
			invite, err := sender.SendInvite(recipient, inviteAmount)
			process.handleError(err, fmt.Sprintf("%v unable to send invite to %v", sender.GetInfo(), u.GetInfo()))
			log.Info(fmt.Sprintf("%s sent invite %s to %s", sender.GetInfo(), invite.Hash, u.GetInfo()))
			invitesCount++
		}
		log.Info(fmt.Sprintf("Sent %v invites", invitesCount))
		return
	}

	log.Info("Start requesting invites")
	var recipients []string
	for _, u := range users {
		if process.validationOnly {
			recipients = append(recipients, u.GetPubKey())
		} else {
			recipients = append(recipients, u.GetAddress())
		}
	}
	delegatees, err := process.apiClient.CreateInvites(recipients)
	process.handleError(err, "Unable to request invites")
	log.Info(fmt.Sprintf("Requested %v invites, delegatees: %v", len(recipients), len(delegatees)))
	for idx, delegatee := range delegatees {
		process.users[idx].SetMultiBotDelegatee(&delegatee)
	}
	return
}

func (process *Process) waitForInvites(users []user.User) {
	process.waitForNodesState(users, invite)
}

func (process *Process) activateInvites(users []user.User) {
	for _, u := range users {
		hash, err := u.ActivateInvite()
		process.handleError(err, fmt.Sprintf("%v unable to activate invite for %v", u.GetInfo(), u.GetAddress()))
		log.Info(fmt.Sprintf("%s sent invite activation %s", u.GetInfo(), hash))
	}
	log.Info("Activated invites")
}

func (process *Process) waitForCandidates(users []user.User) {
	process.waitForNodesState(users, candidate)
}

func (process *Process) waitForNewbies(users []user.User) {
	process.waitForNodesState(users, newbie)
}

func (process *Process) waitForNodesState(users []user.User, state string) {
	log.Info(fmt.Sprintf("Start waiting for user states %v", state))
	wg := sync.WaitGroup{}
	wg.Add(len(users))
	targetStates := []string{state}
	for _, u := range users {
		go func(u user.User) {
			process.waitForNodeState(u, targetStates)
			wg.Done()
		}(u)
	}
	timeoutMin := (process.sc.CeremonyMinOffset + 1) / 2
	if process.sc.CeremonyMinOffset-timeoutMin > 8 {
		timeoutMin = process.sc.CeremonyMinOffset - 8
	}
	ok := common.WaitWithTimeout(&wg, time.Minute*time.Duration(timeoutMin))
	if !ok {
		process.handleError(errors.New(fmt.Sprintf("State %v waiting timeout", state)), "")
	}
	log.Info(fmt.Sprintf("Got state %v for all users", state))
}

func (process *Process) waitForNodeState(u user.User, states []string) {
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

func (process *Process) getIdentity(u user.User) api.Identity {
	identity, err := u.GetIdentity(u.GetAddress())
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
		if u.IsActive() {
			return true
		}
	}
	return false
}

func (process *Process) handleError(err error, prefix string) {
	if err == nil {
		return
	}
	process.mutex.Lock()
	defer process.mutex.Unlock()
	fullPrefix := ""
	if len(prefix) > 0 {
		fullPrefix = fmt.Sprintf("%v: ", prefix)
	}
	fullMessage := fmt.Sprintf("%v%v", fullPrefix, err)
	log.Error(fullMessage)
	process.destroy()
	if !process.godMode {
		if err := process.apiClient.SendFailNotification(fullMessage); err != nil {
			log.Error(errors.Wrap(err, "Unable to send fail notification to god bot").Error())
		}
	}
	os.Exit(1)
}

func (process *Process) handleWarn(message string) {
	log.Warn(message)
}

func (process *Process) switchNodeIfNeeded(u user.User) {
	testIndex := process.getCurrentTestIndex()

	if _, present := process.sc.EpochNodeSwitches[testIndex]; !present {
		return
	}
	if _, present := process.sc.EpochNodeSwitches[testIndex][u.GetIndex()]; !present {
		return
	}

	for _, nodeSwitch := range process.sc.EpochNodeSwitches[testIndex][u.GetIndex()] {
		time.Sleep(nodeSwitch.Delay - time.Now().Sub(u.GetTestContext().TestStartTime))
		if nodeSwitch.IsStart {
			process.startNode(u, node.DeleteNothing)
		} else {
			process.stopNode(u)
		}
	}
}

func waitForMinedTransaction(u user.User, txHash string, timeout time.Duration) error {
	ticker := time.NewTicker(requestRetryDelay)
	defer ticker.Stop()
	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()
	for {
		select {
		case <-ticker.C:
			tx, err := u.Transaction(txHash)
			if err != nil {
				log.Warn(errors.Wrapf(err, "unable to get transaction %v", txHash).Error())
				continue
			}
			if tx.BlockHash == (common2.Hash{}) {
				continue
			}
			return nil
		case <-timeoutTimer.C:
			return errors.Errorf("tx %v is not mined", txHash)
		}
	}
}
