package process

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"idena-test-go/client"
	"idena-test-go/log"
	"idena-test-go/node"
	"idena-test-go/user"
	"time"
)

const (
	firstRpcPort  = 9010
	firstIpfsPort = 4010
	firstPort     = 40410

	defaultFirstCeremonyTimeMinOffset = 3

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
)

type Process struct {
	usersCount        int
	workDir           string
	execCommandName   string
	users             []*user.User
	firstUser         *user.User
	godAddress        string
	bootNode          string
	ipfsBootNode      string
	ceremonyTime      int64
	testCounter       int
	reqIdHolder       *client.ReqIdHolder
	verbosity         int
	maxNetDelay       int
	ceremonyMinOffset int
}

func NewProcess(usersCount int, workDir string, execCommandName string, verbosity int, maxNetDelay int, ceremonyMinOffset int) *Process {
	if ceremonyMinOffset == 0 {
		ceremonyMinOffset = defaultFirstCeremonyTimeMinOffset
	}
	return &Process{
		usersCount:        usersCount,
		workDir:           workDir,
		reqIdHolder:       client.NewReqIdHolder(),
		execCommandName:   execCommandName,
		verbosity:         verbosity,
		maxNetDelay:       maxNetDelay,
		ceremonyMinOffset: ceremonyMinOffset,
	}
}

func (process *Process) Start() {
	defer process.destroy()
	process.init()
	for {
		process.test()
		process.testCounter++
	}
}

func (process *Process) destroy() {
	for _, u := range process.users {
		u.Node.Destroy()
	}
}

func (process *Process) init() {
	log.Debug("Start initializing")

	process.createFirstUser()

	process.startFirstNode()

	process.initGodAddress()

	process.initBootNode()

	process.initIpfsBootNode()

	process.ceremonyTime = process.getCeremonyTime()

	process.restartFirstNode()

	process.createUsers()

	process.startNodes()

	process.getNodeAddresses()

	process.sendInvites()

	process.waitForInvites()

	process.activateInvites()

	process.waitForCandidates()

	log.Debug("Initialization completed")
}

func (process *Process) createFirstUser() {
	n := node.NewNode(process.workDir,
		process.execCommandName,
		DataDir,
		getNodeDataDir(firstRpcPort),
		firstPort,
		true,
		firstRpcPort,
		"",
		"",
		firstIpfsPort,
		"",
		0,
		process.verbosity,
		process.maxNetDelay,
	)
	u := user.NewUser(client.NewClient(*n, process.reqIdHolder), n)
	process.firstUser = u
	process.users = append(process.users, u)
	log.Info("Created first user")
}

func getNodeDataDir(port int) string {
	return fmt.Sprintf("datadir-%d", port)
}

func (process *Process) startFirstNode() {
	process.firstUser.Node.Start(node.DeleteDataDir)
	log.Info("Started first node")
}

func (process *Process) initGodAddress() {
	var err error
	u := process.firstUser
	process.godAddress, err = u.Client.GetCoinbaseAddr()
	process.handleError(err, fmt.Sprintf("%v unable to get address", u.GetInfo()))
	log.Info(fmt.Sprintf("Got god address: %v", process.godAddress))
}

func (process *Process) initBootNode() {
	var err error
	u := process.firstUser
	process.bootNode, err = u.Client.GetEnode()
	process.handleError(err, fmt.Sprintf("%v unable to get enode", u.GetInfo()))
	log.Info(fmt.Sprintf("Got boot node enode: %v", process.bootNode))
}

func (process *Process) initIpfsBootNode() {
	var err error
	u := process.firstUser
	process.ipfsBootNode, err = u.Client.GetIpfsAddress()
	process.handleError(err, fmt.Sprintf("%v unable to get ipfs boot node", u.GetInfo()))
	log.Info(fmt.Sprintf("Got ipfs boot node: %v", process.ipfsBootNode))
}

func (process *Process) restartFirstNode() {
	n := process.firstUser
	n.Node.BootNode = process.bootNode
	n.Node.GodAddress = process.godAddress
	n.Node.IpfsBootNode = process.ipfsBootNode
	n.Node.CeremonyTime = process.ceremonyTime
	n.Node.Start(node.DeleteDb)
	log.Info("Restarted first node")
}

func (process *Process) createUsers() {
	n := process.usersCount - 1
	for i := 0; i < n; i++ {
		process.createUser(i + 1)
	}
	log.Info(fmt.Sprintf("Created %v users", n))
}

func (process *Process) createUser(index int) *user.User {
	rpcPort := firstRpcPort + index
	n := node.NewNode(process.workDir,
		process.execCommandName,
		DataDir,
		getNodeDataDir(rpcPort),
		firstPort+index,
		false,
		rpcPort,
		process.bootNode,
		process.ipfsBootNode,
		firstIpfsPort+index,
		process.godAddress,
		process.ceremonyTime,
		process.verbosity,
		process.maxNetDelay,
	)
	u := user.NewUser(client.NewClient(*n, process.reqIdHolder), n)
	process.users = append(process.users, u)
	log.Info(fmt.Sprintf("%v created", u.GetInfo()))
	return u
}

func (process *Process) startNodes() {
	n := len(process.users) - 1
	for i := 0; i < n; i++ {
		go process.startNode(process.users[i+1].Node)
	}
	time.Sleep(node.NodeStartWaitingTime)
	log.Info(fmt.Sprintf("Started %v nodes", n))
}

func (process *Process) getNodeAddresses() {
	for _, u := range process.users {
		var err error
		u.Address, err = u.Client.GetCoinbaseAddr()
		process.handleError(err, fmt.Sprintf("%v unable to get node address", u.GetInfo()))
		log.Info(fmt.Sprintf("%v got coinbase address %v", u.GetInfo(), u.Address))
	}
}

func (process *Process) startNode(n *node.Node) {
	n.Start(node.DeleteDataDir)
	log.Info(fmt.Sprintf("Started node %v", n.RpcPort))
}

func (process *Process) sendInvites() {
	invitesCount := 0
	for _, u := range process.users {
		_, err := process.firstUser.Client.SendInvite(u.Address)
		process.handleError(err, fmt.Sprintf("%v unable to send invite to %v", u.GetInfo(), u.Address))
		invitesCount++
	}
	log.Info(fmt.Sprintf("Sent %v invites", invitesCount))
}

func (process *Process) waitForInvites() {
	process.waitForNodesState(invite)
}

func (process *Process) activateInvites() {
	for _, u := range process.users {
		_, err := u.Client.ActivateInvite(u.Address)
		process.handleError(err, fmt.Sprintf("%v unable to activate invite for %v", u.GetInfo(), u.Address))
	}
	log.Info("Activated invites")
}

func (process *Process) waitForCandidates() {
	process.waitForNodesState(candidate)
}

func (process *Process) waitForNodesState(state string) {
	log.Info(fmt.Sprintf("Start waiting for user states %v", state))
	timeout := time.NewTimer(stateWaitingTimeout)
	defer timeout.Stop()
	ch := make(chan struct{}, process.usersCount)
	for _, u := range process.users {
		go process.waitForNodeStateWithSignal(u, []string{state}, ch)
	}
	target := process.usersCount
	counter := 0
	for counter < target {
		select {
		case <-ch:
			counter++
		case <-timeout.C:
			process.handleError(errors.New(fmt.Sprintf("State %v waiting timeout", state)), "")
		}
	}
	log.Info(fmt.Sprintf("Got state %v for all users", state))
}

func (process *Process) waitForNodeStateWithSignal(u *user.User, states []string, ch chan struct{}) {
	process.waitForNodeState(u, states)
	ch <- struct{}{}
}

func (process *Process) waitForNodeState(u *user.User, states []string) {
	log.Info(fmt.Sprintf("%v start waiting for one of the states %v", u.GetInfo(), states))
	var currentState string
	for {
		identity, err := u.Client.GetIdentity(u.Address)
		process.handleError(err, fmt.Sprintf("%v unable to get identities", u.GetInfo()))
		currentState = identity.State
		log.Debug(fmt.Sprintf("%v state %v", u.GetInfo(), currentState))
		if in(currentState, states) {
			break
		}
		time.Sleep(requestRetryDelay)
	}
	log.Info(fmt.Sprintf("%v got target state %v", u.GetInfo(), currentState))
}

func in(value string, list []string) bool {
	for _, elem := range list {
		if elem == value {
			return true
		}
	}
	return false
}

func (process *Process) test() {
	log.Info(fmt.Sprintf("************** Start waiting for verification sessions (test #%v) **************", process.testCounter))
	timeout := time.NewTimer(process.getTestTimeout())
	defer timeout.Stop()
	ch := make(chan struct{}, len(process.users))
	for _, u := range process.users {
		go process.testUser(u, process.godAddress, ch)
	}
	target := len(process.users)
	counter := 0
	for counter < target {
		select {
		case <-ch:
			counter++
		case <-timeout.C:
			process.handleError(errors.New("verification sessions timeout"), "")
		}
	}
	log.Info(fmt.Sprintf("************** All verification sessions completed (test #%v) **************", process.testCounter))
}

func (process *Process) getTestTimeout() time.Duration {
	u := process.firstUser
	epoch, err := u.Client.GetEpoch()
	process.handleError(err, fmt.Sprintf("%v unable to get epoch", u.GetInfo()))
	intervals, err := u.Client.CeremonyIntervals()
	process.handleError(err, fmt.Sprintf("%v unable to get ceremony intervals", u.GetInfo()))
	now := time.Now()
	nextValidation := epoch.NextValidation
	testTimeout := nextValidation.Sub(now) +
		time.Second*time.Duration(intervals.FlipLotteryDuration) +
		time.Second*time.Duration(intervals.ShortSessionDuration) +
		time.Second*time.Duration(intervals.LongSessionDuration) +
		time.Second*time.Duration(intervals.AfterLongSessionDuration) +
		time.Minute*3
	log.Debug(fmt.Sprintf("Verification session waiting timeout: %v", testTimeout))
	return testTimeout
}

func (process *Process) testUser(u *user.User, godAddress string, ch chan struct{}) {
	process.initTest(u)

	process.passVerification(u, godAddress)

	waitForSessionFinish(u)

	log.Info(fmt.Sprintf("%v passed verification session", u.GetInfo()))

	ch <- struct{}{}
}

func (process *Process) passVerification(u *user.User, godAddress string) {
	if !process.submitFlips(u, godAddress) {
		log.Warn(fmt.Sprintf("%v didn't manage to submit all required flips, verification will be missed", u.GetInfo()))
		return
	}

	waitForShortSession(u)

	log.Debug(fmt.Sprintf("%v required flips count: %d", u.GetInfo(), process.getRequiredFlipsCount(u)))

	process.getShortFlipHashes(u)

	process.getShortFlips(u)

	process.submitShortAnswers(u)

	waitForLongSession(u)

	process.getLongFlipHashes(u)

	process.getLongFlips(u)

	process.submitLongAnswers(u)

	process.waitForNodeState(u, []string{newbie, verified})
}

func (process *Process) initTest(u *user.User) {
	u.ShortFlipHashes = nil
	u.ShortFlips = nil
	u.LongFlipHashes = nil
	u.LongFlips = nil
}

func (process *Process) submitFlips(u *user.User, godAddress string) bool {
	requiredFlipsCount := process.getRequiredFlipsCount(u)
	if u.Address == godAddress && requiredFlipsCount == 0 {
		requiredFlipsCount = 5
	}
	log.Info(fmt.Sprintf("%v required flips count: %v", u.GetInfo(), requiredFlipsCount))
	if requiredFlipsCount == 0 {
		return true
	}
	submittedFlipsCount := 0
	for i := 0; i < requiredFlipsCount; i++ {
		flipHex, err := randomHex(10)
		if err != nil {
			process.handleError(err, "unable to generate hex")
		}
		_, err = u.Client.SubmitFlip(flipHex)
		if err != nil {
			log.Error(fmt.Sprintf("%v unable to submit flip: %v", u.GetInfo(), err))
		} else {
			log.Debug(fmt.Sprintf("%v submitted flip", u.GetInfo()))
			submittedFlipsCount++
		}
	}
	log.Info(fmt.Sprintf("%v submitted %v flips", u.GetInfo(), submittedFlipsCount))
	return submittedFlipsCount == requiredFlipsCount
}

func (process *Process) getRequiredFlipsCount(u *user.User) int {
	identity, err := u.Client.GetIdentity(u.Address)
	process.handleError(err, fmt.Sprintf("%v unable to get identities", u.GetInfo()))
	requiredFlipsCount := identity.RequiredFlips - identity.MadeFlips
	return requiredFlipsCount
}

func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return "0x" + hex.EncodeToString(bytes), nil
}

func waitForShortSession(u *user.User) {
	waitForPeriod(u, periodShortSession)
}

func waitForPeriod(u *user.User, period string) {
	log.Info(fmt.Sprintf("%v start waiting for period %v", u.GetInfo(), period))
	for {
		epoch, _ := u.Client.GetEpoch()
		currentPeriod := epoch.CurrentPeriod
		log.Debug(fmt.Sprintf("%v current period: %v", u.GetInfo(), currentPeriod))
		if currentPeriod == period {
			break
		}
		time.Sleep(requestRetryDelay)
	}
	log.Info(fmt.Sprintf("%v period %v started", u.GetInfo(), period))
}

func (process *Process) getShortFlipHashes(u *user.User) {
	deadline := time.Now().Add(flipsWaitingTimeout)
	for time.Now().Before(deadline) {
		shortFlipHashes, err := u.Client.GetShortFlipHashes()
		if err != nil || !process.checkFlipHashes(shortFlipHashes) {
			time.Sleep(requestRetryDelay)
			continue
		}
		u.ShortFlipHashes = shortFlipHashes
		log.Info(fmt.Sprintf("%v got %d short flip hashes", u.GetInfo(), len(shortFlipHashes)))
		return
	}
	process.handleError(errors.New(fmt.Sprintf("%v short flip hashes waiting timeout", u.GetInfo())), "")
}

func (process *Process) checkFlipHashes(flipHashes []client.FlipHashesResponse) bool {
	if len(flipHashes) == 0 {
		return false
	}
	for _, f := range flipHashes {
		if !f.Ready {
			return false
		}
	}
	return true
}

func (process *Process) getShortFlips(u *user.User) {
	deadline := time.Now().Add(flipsWaitingTimeout)
	flipsByHash := make(map[string]*client.FlipResponse)
	for time.Now().Before(deadline) {
		for _, h := range u.ShortFlipHashes {
			if flipsByHash[h.Hash] != nil {
				continue
			}
			flipResponse, err := u.Client.GetFlip(h.Hash)
			if err != nil {
				continue
			}
			flipsByHash[h.Hash] = &flipResponse
		}
		if len(flipsByHash) == len(u.ShortFlipHashes) {
			var flips []client.FlipResponse
			for _, f := range flipsByHash {
				flips = append(flips, *f)
			}
			u.ShortFlips = flips
			log.Info(fmt.Sprintf("%v got %v short flips", u.GetInfo(), len(flips)))
			return
		}
		time.Sleep(requestRetryDelay)
	}
	process.handleError(errors.New(fmt.Sprintf("%v short flips waiting timeout", u.GetInfo())), "")
}

func (process *Process) submitShortAnswers(u *user.User) {
	var answers []byte
	for range u.ShortFlips {
		answers = append(answers, 1)
	}
	_, err := u.Client.SubmitShortAnswers(answers)
	process.handleError(err, fmt.Sprintf("%v unable to submit short answers", u.GetInfo()))
	log.Info(fmt.Sprintf("%v submitted %d short answers: %v", u.GetInfo(), len(answers), answers))
}

func waitForLongSession(u *user.User) {
	waitForPeriod(u, periodLongSession)
}

func (process *Process) getLongFlipHashes(u *user.User) {
	var err error
	u.LongFlipHashes, err = u.Client.GetLongFlipHashes()
	process.handleError(err, fmt.Sprintf("%v unable to load long flip hashes", u.GetInfo()))
	log.Info(fmt.Sprintf("%v got %d long flip hashes", u.GetInfo(), len(u.LongFlipHashes)))
}

func (process *Process) getLongFlips(u *user.User) {
	u.LongFlips = process.getFlips(u, u.LongFlipHashes)
	log.Info(fmt.Sprintf("%v got %d long flips", u.GetInfo(), len(u.LongFlips)))
}

func (process *Process) getFlips(u *user.User, flipHashes []client.FlipHashesResponse) []client.FlipResponse {
	var result []client.FlipResponse
	for _, flipHashResponse := range flipHashes {
		flipResponse, err := u.Client.GetFlip(flipHashResponse.Hash)
		process.handleError(err, fmt.Sprintf("%v unable to get flip %v", u.GetInfo(), flipHashResponse.Hash))
		result = append(result, flipResponse)
	}
	return result
}

func (process *Process) submitLongAnswers(u *user.User) {
	var answers []byte
	for range u.LongFlips {
		answers = append(answers, 1)
	}
	_, err := u.Client.SubmitLongAnswers(answers)
	process.handleError(err, fmt.Sprintf("%v unable to submit long answers", u.GetInfo()))
	log.Info(fmt.Sprintf("%v submitted %d long answers: %v", u.GetInfo(), len(answers), answers))
}

func waitForSessionFinish(u *user.User) {
	waitForPeriod(u, periodNone)
}

func (process *Process) getCeremonyTime() int64 {
	return time.Now().UTC().Unix() + int64(process.ceremonyMinOffset*60)
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
