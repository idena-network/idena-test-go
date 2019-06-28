package process

import (
	"encoding/hex"
	"errors"
	"fmt"
	"idena-test-go/client"
	"idena-test-go/common"
	"idena-test-go/log"
	"idena-test-go/node"
	"idena-test-go/scenario"
	"idena-test-go/user"
	"math/rand"
	"sync"
	"time"
)

func (process *Process) test() {
	log.Info(fmt.Sprintf("************** Start waiting for verification sessions (test #%v) **************", process.getCurrentTestIndex()))
	wg := &sync.WaitGroup{}
	wg.Add(len(process.users))
	timeout := process.getTestTimeout()
	es := epochState{
		userStates: make(map[int]*userEpochState),
	}
	mutex := sync.Mutex{}
	for _, u := range process.users {
		go func(u *user.User) {
			ues := &userEpochState{}
			process.testUser(u, process.godAddress, ues)
			mutex.Lock()
			es.userStates[u.Index] = ues
			mutex.Unlock()
			wg.Done()
		}(u)
	}

	process.startEpochBackgroundProcess(wg, timeout)

	ok := common.WaitWithTimeout(wg, timeout)
	if !ok {
		process.handleError(errors.New("verification sessions timeout"), "")
	}
	process.assert(process.getCurrentTestIndex(), es)
	log.Info(fmt.Sprintf("************** All verification sessions completed (test #%d) **************", process.getCurrentTestIndex()))
}

func (process *Process) getTestTimeout() time.Duration {
	u := process.getActiveUsers()[0]
	epoch := process.getEpoch(u)
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

func (process *Process) testUser(u *user.User, godAddress string, state *userEpochState) {
	process.initTest(u)

	if !u.Active {
		log.Info(fmt.Sprintf("%v skipped verification session due to stopped node", u.GetInfo()))
		return
	}

	epoch := process.getEpoch(u)
	log.Info(fmt.Sprintf("%s epoch: %d, next validation time: %v", u.GetInfo(), epoch.Epoch, epoch.NextValidation))

	process.syncAllBotsNewEpoch()

	process.switchOnlineState(u, epoch.NextValidation)

	process.submitFlips(u, godAddress)

	process.provideDelayedFlipKeyIfNeeded(u, epoch.NextValidation)

	waitForShortSession(u)

	process.passVerification(u, godAddress)

	process.collectUserEpochState(u, state)

	waitForSessionFinish(u)

	log.Info(fmt.Sprintf("%v passed verification session", u.GetInfo()))
}

func (process *Process) syncAllBotsNewEpoch() {
	if process.godMode && process.getCurrentTestIndex() == 0 {
		return
	}
	d := time.Second * 90
	time.Sleep(d)
}

func (process *Process) switchOnlineState(u *user.User, nextValidationTime time.Time) {
	testIndex := process.getCurrentTestIndex()
	onlines := process.sc.EpochNodeOnlines[testIndex]
	becomeOnline := pos(onlines, u.Index) != -1
	if becomeOnline {
		process.tryToSwitchOnlineState(u, nextValidationTime, true)
	}
	offlines := process.sc.EpochNodeOfflines[testIndex]
	becomeOffline := pos(offlines, u.Index) != -1
	if becomeOffline {
		process.tryToSwitchOnlineState(u, nextValidationTime, false)
	}

	// Become online by default if state is newbie
	if !becomeOnline && !becomeOffline && !u.SentAutoOnline {
		identity := process.getIdentity(u)
		if identity.State == newbie {
			process.tryToSwitchOnlineState(u, nextValidationTime, true)
			u.SentAutoOnline = true
		}
	}
}

func (process *Process) tryToSwitchOnlineState(u *user.User, nextValidationTime time.Time, online bool) {
	var switchOnline func() (string, error)
	var stateName string
	if online {
		switchOnline = u.Client.BecomeOnline
		stateName = "online"
	} else {
		switchOnline = u.Client.BecomeOffline
		stateName = "offline"
	}
	// Try to switch online state till (nextValidationTime - 1 minute) to leave time for submitting flips
	deadline := nextValidationTime.Add(-time.Minute)
	attempts := 0
	for {
		hash, err := switchOnline()
		attempts++
		if err == nil {
			log.Info(fmt.Sprintf("%v sent request to become %s, tx: %s, attempts: %d", u.GetInfo(), stateName, hash, attempts))
			return
		}
		if time.Now().After(deadline) {
			process.handleError(err, fmt.Sprintf("%v unable to become %s, attempts: %d", u.GetInfo(), stateName, attempts))
			return
		}
		time.Sleep(requestRetryDelay)
	}
}

func pos(slice []int, target int) int {
	for i, v := range slice {
		if v == target {
			return i
		}
	}
	return -1
}

func (process *Process) collectUserEpochState(u *user.User, state *userEpochState) {
	identity := process.getIdentity(u)
	state.madeFlips = len(identity.Flips)
	state.requiredFlips = identity.RequiredFlips
}

func (process *Process) passVerification(u *user.User, godAddress string) {

	log.Debug(fmt.Sprintf("%v required flips: %d", u.GetInfo(), process.getRequiredFlipsCount(u)))

	process.getFlipHashes(u, true)

	process.getFlips(u, true)

	process.submitAnswers(u, true)

	waitForLongSession(u)

	process.getFlipHashes(u, false)

	process.getFlips(u, false)

	process.submitAnswers(u, false)
}

func (process *Process) initTest(u *user.User) {
	ctx := user.TestContext{}
	ctx.ShortFlipHashes = nil
	ctx.ShortFlips = nil
	ctx.LongFlipHashes = nil
	ctx.LongFlips = nil
	u.TestContext = &ctx
}

func (process *Process) submitFlips(u *user.User, godAddress string) {
	flipsToSubmit := process.getFlipsCountToSubmit(u, godAddress)
	if flipsToSubmit == 0 {
		return
	}
	var submittedFlipHashes []string
	var submittedFlipTxs []string
	for i := 0; i < flipsToSubmit; i++ {
		flipHex, err := randomHex(10)
		if err != nil {
			process.handleError(err, "unable to generate hex")
		}
		resp, err := u.Client.SubmitFlip(flipHex)
		if err != nil {
			log.Error(fmt.Sprintf("%v unable to submit flip: %v", u.GetInfo(), err))
		} else {
			log.Debug(fmt.Sprintf("%v submitted flip", u.GetInfo()))
			submittedFlipHashes = append(submittedFlipHashes, resp.Hash)
			submittedFlipTxs = append(submittedFlipTxs, resp.TxHash)
		}
	}
	log.Info(fmt.Sprintf("%v submitted %v flips: %v, txs: %v", u.GetInfo(), len(submittedFlipHashes), submittedFlipHashes, submittedFlipTxs))
}

func (process *Process) getFlipsCountToSubmit(u *user.User, godAddress string) int {
	requiredFlipsCount := process.getRequiredFlipsCount(u)
	if u.Address == godAddress && requiredFlipsCount == 0 {
		requiredFlipsCount = initialRequiredFlips
	}
	log.Info(fmt.Sprintf("%v required flips: %v", u.GetInfo(), requiredFlipsCount))

	flipsCountToSubmit := requiredFlipsCount
	userCeremony := process.getScUserCeremony(u)
	if userCeremony != nil {
		flipsCountToSubmit = userCeremony.SubmitFlips
	}

	return flipsCountToSubmit
}

func (process *Process) getScUserCeremony(u *user.User) *scenario.UserCeremony {
	ceremony := process.sc.Ceremonies[process.getCurrentTestIndex()]
	if ceremony == nil {
		return nil
	}
	return ceremony.UserCeremonies[u.Index]
}

func (process *Process) getRequiredFlipsCount(u *user.User) int {
	identity := process.getIdentity(u)
	requiredFlipsCount := identity.RequiredFlips - len(identity.Flips)
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

func waitForLongSession(u *user.User) {
	waitForPeriod(u, periodLongSession)
}

func waitForSessionFinish(u *user.User) {
	waitForPeriod(u, periodNone)
}

func (process *Process) getCurrentTestIndex() int {
	return process.testCounter
}

func (process *Process) getFlipHashes(u *user.User, isShort bool) {
	name := getSessionName(isShort)
	var loadFunc func() ([]client.FlipHashesResponse, error)
	var setFunc func([]client.FlipHashesResponse)
	if isShort {
		loadFunc = u.Client.GetShortFlipHashes
		setFunc = func(flipHashes []client.FlipHashesResponse) {
			u.TestContext.ShortFlipHashes = flipHashes
		}
	} else {
		loadFunc = u.Client.GetLongFlipHashes
		setFunc = func(flipHashes []client.FlipHashesResponse) {
			u.TestContext.LongFlipHashes = flipHashes
		}
	}
	// Random deadline to have some users with None answers in case of delayed flip keys
	deadlineOffset := flipsWaitingMinTimeout +
		time.Duration(rand.Int63n(int64(flipsWaitingMaxTimeout-flipsWaitingMinTimeout)))
	deadline := time.Now().Add(deadlineOffset)
	var flipHashes []client.FlipHashesResponse
	for {
		var err error
		flipHashes, err = loadFunc()
		if err != nil {
			process.handleError(err, fmt.Sprintf("%v unable to get %s flip hashes", u.GetInfo(), name))
			return
		}
		err = process.checkFlipHashes(flipHashes)
		if err == nil {
			break
		}
		log.Info(fmt.Sprintf("%v unable to get %s flip hashes: %v", u.GetInfo(), name, err))
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(requestRetryDelay)
	}
	setFunc(flipHashes)
	log.Info(fmt.Sprintf("%v got %d %s flip hashes", u.GetInfo(), len(flipHashes), name))
}

func (process *Process) checkFlipHashes(flipHashes []client.FlipHashesResponse) error {
	if len(flipHashes) == 0 {
		return errors.New("empty flip hashes")
	}
	for _, f := range flipHashes {
		if !f.Ready {
			return errors.New(fmt.Sprintf("Not ready flip %v", f.Hash))
		}
	}
	return nil
}

func (process *Process) getFlips(u *user.User, isShort bool) {
	name := getSessionName(isShort)
	var flipHashes []client.FlipHashesResponse
	var setFunc func([]client.FlipResponse)
	if isShort {
		flipHashes = u.TestContext.ShortFlipHashes
		setFunc = func(flips []client.FlipResponse) {
			u.TestContext.ShortFlips = flips
		}
	} else {
		flipHashes = u.TestContext.LongFlipHashes
		setFunc = func(flips []client.FlipResponse) {
			u.TestContext.LongFlips = flips
		}
	}

	var flips []client.FlipResponse
	for _, h := range flipHashes {
		if !h.Ready {
			flips = append(flips, emptyFlip())
			continue
		}
		flipResponse, err := u.Client.GetFlip(h.Hash)
		if err != nil {
			process.handleError(err, fmt.Sprintf("%v unable to get flip %s", u.GetInfo(), h.Hash))
			continue
		}
		flips = append(flips, flipResponse)
	}
	setFunc(flips)
	log.Info(fmt.Sprintf("%v got %v %s flips", u.GetInfo(), len(flips), name))
	return
}

func emptyFlip() client.FlipResponse {
	return client.FlipResponse{}
}

func (process *Process) submitAnswers(u *user.User, isShort bool) {
	var submitFunc func([]byte) (client.SubmitAnswersResponse, error)
	name := getSessionName(isShort)
	if isShort {
		submitFunc = u.Client.SubmitShortAnswers
	} else {
		submitFunc = u.Client.SubmitLongAnswers
	}
	answers := process.getAnswers(u, isShort)
	resp, err := submitFunc(answers)
	process.handleError(err, fmt.Sprintf("%v unable to submit %s answers", u.GetInfo(), name))
	log.Info(fmt.Sprintf("%v submitted %d %s answers: %v, tx: %s", u.GetInfo(), len(answers), name, answers, resp.TxHash))
}

func (process *Process) getAnswers(u *user.User, isShort bool) []byte {
	var answersHolder scenario.AnswersHolder
	var flips []client.FlipResponse
	userCeremony := process.getScUserCeremony(u)
	if isShort {
		if userCeremony != nil {
			answersHolder = userCeremony.ShortAnswers
		}
		flips = u.TestContext.ShortFlips
	} else {
		if userCeremony != nil {
			answersHolder = userCeremony.LongAnswers
		}
		flips = u.TestContext.LongFlips
	}

	var answers []byte
	if answersHolder != nil {
		answers = answersHolder.Get(len(flips))
	} else {
		for range flips {
			answers = append(answers, process.sc.DefaultAnswer)
		}
	}
	process.setNoneAnswers(flips, answers)
	return answers
}

func (process *Process) setNoneAnswers(flips []client.FlipResponse, answers []byte) {
	emptyFlip := emptyFlip()
	for i, flip := range flips {
		if flip == emptyFlip {
			answers[i] = common.None
		}
	}
}

func getSessionName(isShort bool) string {
	if isShort {
		return "short"
	}
	return "long"
}

func (process *Process) provideDelayedFlipKeyIfNeeded(u *user.User, nextValidationTime time.Time) {
	users, present := process.sc.EpochDelayedFlipKeys[process.getCurrentTestIndex()]
	if !present || pos(users, u.Index) == -1 {
		return
	}
	log.Info(fmt.Sprintf("%v providing delayed flip key", u.GetInfo()))
	sleepTime := nextValidationTime.Sub(time.Now()) + shortSessionFlipKeyDeadline + time.Second*5
	time.Sleep(time.Second * 20) // Time for mining last operations
	go process.stopNode(u)
	time.Sleep(sleepTime)
	process.startNode(u, node.DeleteNothing)
}

func (process *Process) getEpoch(u *user.User) client.Epoch {
	epoch, err := u.Client.GetEpoch()
	process.handleError(err, fmt.Sprintf("%v unable to get epoch", u.GetInfo()))
	return epoch
}
