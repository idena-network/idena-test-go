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
	"os"
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
	testTimeout         = 11 * time.Minute
	submitFlipDelay     = 5 * time.Second
)

type Process struct {
	usersCount        int
	workDir           string
	execCommandName   string
	users             []*user.User
	firstUser         *user.User
	godAddress        string
	bootNode          string
	ceremonyTime      int64
	testCounter       int
	reqIdHolder       *client.ReqIdHolder
	verbosity         int
	ceremonyMinOffset int
}

func NewProcess(usersCount int, workDir string, execCommandName string, verbosity int, ceremonyMinOffset int) *Process {
	if ceremonyMinOffset == 0 {
		ceremonyMinOffset = defaultFirstCeremonyTimeMinOffset
	}
	return &Process{
		usersCount:        usersCount,
		workDir:           workDir,
		reqIdHolder:       client.NewReqIdHolder(),
		execCommandName:   execCommandName,
		verbosity:         verbosity,
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

	process.createWorkDir()

	process.createFirstUser()

	process.startFirstNode()

	process.getGodAddress()

	process.getBootNode()

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

func (process *Process) createWorkDir() {
	if _, err := os.Stat(process.workDir); os.IsNotExist(err) {
		err := os.MkdirAll(process.workDir, os.ModePerm)
		if err != nil {
			process.handleError(err, "Unable to create dir")
		}
	}
}

func (process *Process) createFirstUser() {
	n := node.NewNode(process.workDir,
		process.execCommandName,
		"datadir-automine",
		firstPort,
		true,
		firstRpcPort,
		process.bootNode,
		firstIpfsPort,
		process.godAddress,
		0,
		process.verbosity,
	)
	u := user.NewUser(client.NewClient(*n, process.reqIdHolder), n, 0)
	process.firstUser = u
	process.users = append(process.users, u)
	log.Info("Created first user")
}

func (process *Process) startFirstNode() {
	process.firstUser.Node.Start(node.DeleteDataDir)
	log.Info("Started first node")
}

func (process *Process) getGodAddress() {
	var err error
	u := process.firstUser
	process.godAddress, err = u.Client.GetCoinbaseAddr()
	process.handleError(err, fmt.Sprintf("%v, unable to get address", u.GetInfo()))
	log.Info(fmt.Sprintf("Got god address: %v", process.godAddress))
}

func (process *Process) getBootNode() {
	var err error
	u := process.firstUser
	process.bootNode, err = u.Client.GetEnode()
	process.handleError(err, fmt.Sprintf("%v, unable to get enode", u.GetInfo()))
	log.Info(fmt.Sprintf("Got boot node enode: %v", process.bootNode))
}

func (process *Process) restartFirstNode() {
	n := process.firstUser
	n.Node.BootNode = process.bootNode
	n.Node.GodAddress = process.godAddress
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
	n := node.NewNode(process.workDir,
		process.execCommandName,
		fmt.Sprintf("datadir-%v", index),
		firstPort+index,
		false,
		firstRpcPort+index,
		process.bootNode,
		firstIpfsPort+index,
		process.godAddress,
		process.ceremonyTime,
		process.verbosity,
	)
	u := user.NewUser(client.NewClient(*n, process.reqIdHolder), n, index)
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
		process.handleError(err, fmt.Sprintf("%v, unable to get node address", u.GetInfo()))
		log.Info(fmt.Sprintf("%v, got address %v", u.GetInfo(), u.Address))
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
		process.handleError(err, fmt.Sprintf("%v, unable to send invite to %v", u.GetInfo(), u.Address))
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
		process.handleError(err, fmt.Sprintf("%v, unable to activate invite for %v", u.GetInfo(), u.Address))
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
	log.Info(fmt.Sprintf("%v, start waiting for one of the states %v", u.GetInfo(), states))
	var currentState string
	for {
		identities, err := u.Client.GetIdentities()
		process.handleError(err, fmt.Sprintf("%v, unable to get identities", u.GetInfo()))
		identity := getNodeIdentity(identities, u.Address)
		if identity != nil {
			currentState = identity.State
		}
		log.Debug(fmt.Sprintf("%v, state %v", u.GetInfo(), currentState))
		if in(currentState, states) {
			break
		}
		time.Sleep(2 * time.Second)
	}
	log.Info(fmt.Sprintf("%v, got target state %v", u.GetInfo(), currentState))
}

func in(value string, list []string) bool {
	for _, elem := range list {
		if elem == value {
			return true
		}
	}
	return false
}

func getNodeIdentity(identites []client.Identity, nodeAddress string) *client.Identity {
	for _, identity := range identites {
		if identity.Address == nodeAddress {
			return &identity
		}
	}
	return nil
}

func (process *Process) test() {
	log.Info(fmt.Sprintf("Start waiting for verification sessions (test #%v)", process.testCounter))
	timeout := time.NewTimer(testTimeout)
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
	log.Info(fmt.Sprintf("All verification sessions completed (test #%v)", process.testCounter))
}

func (process *Process) testUser(u *user.User, godAddress string, ch chan struct{}) {
	process.submitFlips(u, godAddress)

	waitForShortSession(u)

	process.getShortFlipHashes(u)

	process.getShortFlips(u)

	process.submitShortAnswers(u)

	waitForLongSession(u)

	process.getLongFlipHashes(u)

	process.getLongFlips(u)

	process.submitLongAnswers(u)

	process.waitForNodeState(u, []string{newbie, verified})

	waitForSessionFinish(u)

	ch <- struct{}{}
}

func (process *Process) submitFlips(u *user.User, godAddress string) {
	requiredFlipsCount := process.getRequiredFlipsCount(u, godAddress, true)
	log.Info(fmt.Sprintf("%v, required flips count: %v", u.GetInfo(), requiredFlipsCount))
	if requiredFlipsCount == 0 {
		return
	}
	submittedFlipsCount := 0
	for requiredFlipsCount > 0 {
		for i := 0; i < requiredFlipsCount; i++ {
			flipHex, err := randomHex(10)
			if err != nil {
				process.handleError(err, "unable to generate hex")
			}
			_, err = u.Client.SubmitFlip(flipHex)
			if err != nil {
				log.Error(fmt.Sprintf("%v, unable to submit flip", u.GetInfo()))
			} else {
				submittedFlipsCount++
			}
			time.Sleep(submitFlipDelay)
		}
		requiredFlipsCount = process.getRequiredFlipsCount(u, godAddress, false)
	}
	log.Info(fmt.Sprintf("%v, submitted %v flips", u.GetInfo(), submittedFlipsCount))
}

func (process *Process) getRequiredFlipsCount(u *user.User, godAddress string, isFirst bool) int {
	var defaultValue int
	if u.Address == godAddress && isFirst {
		defaultValue = 5
	}
	identities, err := u.Client.GetIdentities()
	process.handleError(err, fmt.Sprintf("%v, unable to get identities", u.GetInfo()))
	identity := getNodeIdentity(identities, u.Address)
	requiredFlipsCount := identity.RequiredFlips - identity.MadeFlips
	if requiredFlipsCount == 0 {
		requiredFlipsCount = defaultValue
	}
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
	log.Info(fmt.Sprintf("%v, start waiting for period %v", u.GetInfo(), period))
	for {
		epoch, _ := u.Client.GetEpoch()
		currentPeriod := epoch.CurrentPeriod
		log.Debug(fmt.Sprintf("%v, current period: %v", u.GetInfo(), currentPeriod))
		if currentPeriod == period {
			break
		}
		time.Sleep(2 * time.Second)
	}
	log.Info(fmt.Sprintf("%v, period %v started", u.GetInfo(), period))
}

func (process *Process) getShortFlipHashes(u *user.User) {
	var err error
	u.ShortFlipHashes, err = u.Client.GetShortFlipHashes()
	process.handleError(err, fmt.Sprintf("%v, unable to load short flip hashes", u.GetInfo()))
	log.Info(fmt.Sprintf("%v, got short flip hashes", u.GetInfo()))
}

func (process *Process) getShortFlips(u *user.User) {
	u.ShortFlips = process.getFlips(u, u.ShortFlipHashes)
	log.Info(fmt.Sprintf("%v, got short flips", u.GetInfo()))
}

func (process *Process) getFlips(u *user.User, flipHashes []client.FlipHashesResponse) []client.FlipResponse {
	var result []client.FlipResponse
	for _, flipHashResponse := range flipHashes {
		flipResponse, err := u.Client.GetFlip(flipHashResponse.Hash)
		process.handleError(err, fmt.Sprintf("%v, unable to get flip %v", u.GetInfo(), flipHashResponse.Hash))
		result = append(result, flipResponse)
	}
	return result
}

func (process *Process) submitShortAnswers(u *user.User) {
	var answers []byte
	for range u.ShortFlips {
		answers = append(answers, 1)
	}
	_, err := u.Client.SubmitShortAnswers(answers)
	process.handleError(err, fmt.Sprintf("%v, unable to submit long answers", u.GetInfo()))
	log.Info(fmt.Sprintf("%v, submitted short answers", u.GetInfo()))
}

func waitForLongSession(u *user.User) {
	waitForPeriod(u, periodLongSession)
}

func (process *Process) getLongFlipHashes(u *user.User) {
	var err error
	u.LongFlipHashes, err = u.Client.GetLongFlipHashes()
	process.handleError(err, fmt.Sprintf("%v, unable to load long flip hashes", u.GetInfo()))
	log.Info(fmt.Sprintf("%v, got long flip hashes", u.GetInfo()))
}

func (process *Process) getLongFlips(u *user.User) {
	u.LongFlips = process.getFlips(u, u.LongFlipHashes)
	log.Info(fmt.Sprintf("%v, got long flips", u.GetInfo()))
}

func (process *Process) submitLongAnswers(u *user.User) {
	var answers []byte
	for range u.LongFlips {
		answers = append(answers, 1)
	}
	_, err := u.Client.SubmitLongAnswers(answers)
	process.handleError(err, fmt.Sprintf("%v, unable to submit long answers", u.GetInfo()))
	log.Info(fmt.Sprintf("%v, submitted long answers", u.GetInfo()))
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
