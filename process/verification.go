package process

import (
	"encoding/hex"
	"fmt"
	mapset "github.com/deckarep/golang-set"
	common2 "github.com/idena-network/idena-go/common"
	"github.com/idena-network/idena-test-go/client"
	"github.com/idena-network/idena-test-go/common"
	"github.com/idena-network/idena-test-go/log"
	"github.com/idena-network/idena-test-go/node"
	"github.com/idena-network/idena-test-go/scenario"
	"github.com/idena-network/idena-test-go/user"
	"github.com/pkg/errors"
	"math/rand"
	"strings"
	"sync"
	"time"
)

const (
	skipSessionMessageFormat = "%v skipped verification session due to stopped node"
)

func (process *Process) test() {
	log.Info(fmt.Sprintf("************** Start waiting for verification sessions (test #%v) **************", process.getCurrentTestIndex()))

	process.waitForGodBotNewEpoch()

	wg := &sync.WaitGroup{}
	wg.Add(len(process.users))
	process.initCeremonyIntervals()
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
		var nodeNames []string
		for _, u := range process.users {
			if u.IsTestRun {
				nodeNames = append(nodeNames, u.GetInfo())
			}
		}
		process.handleError(errors.New("verification sessions timeout"), strings.Join(nodeNames, ","))
	}
	process.assert(process.getCurrentTestIndex(), es)
	log.Info(fmt.Sprintf("************** All verification sessions completed (test #%d) **************", process.getCurrentTestIndex()))
}

func (process *Process) initCeremonyIntervals() {
	u := process.getActiveUsers()[0]
	intervals, err := u.Client.CeremonyIntervals()
	process.handleError(err, fmt.Sprintf("%v unable to get ceremony intervals", u.GetInfo()))
	process.ceremonyIntervals = &intervals
}

func (process *Process) getTestTimeout() time.Duration {
	u := process.getActiveUsers()[0]
	epoch := process.getEpoch(u)
	intervals := process.ceremonyIntervals
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
	u.IsTestRun = true
	defer func() {
		u.IsTestRun = false
	}()
	process.initTest(u)

	wasActive := u.Active

	if !wasActive {
		process.switchNodeIfNeeded(u)
	}

	if !u.Active {
		log.Info(fmt.Sprintf(skipSessionMessageFormat, u.GetInfo()))
		return
	}

	epoch := process.getEpoch(u)
	log.Info(fmt.Sprintf("%s epoch: %d, next validation time: %v", u.GetInfo(), epoch.Epoch, epoch.NextValidation))

	process.switchOnlineState(u, epoch.NextValidation)

	if wasActive {
		process.switchNodeIfNeeded(u)
	}

	if !u.Active {
		log.Info(fmt.Sprintf(skipSessionMessageFormat, u.GetInfo()))
		return
	}

	process.submitFlips(u, godAddress)

	process.provideDelayedFlipKeyIfNeeded(u, epoch.NextValidation)

	waitForShortSession(u)

	process.passVerification(u)

	process.collectUserEpochState(u, state)

	waitForSessionFinish(u)
}

func (process *Process) waitForGodBotNewEpoch() {
	if process.godMode {
		return
	}
	u := process.getActiveUsers()[0]
	epoch := process.getEpoch(u).Epoch
	log.Info(fmt.Sprintf("Start waiting for god bot epoch %d", epoch))
	for {
		godBotEpoch, err := process.apiClient.GetEpoch()
		if err == nil && godBotEpoch == epoch {
			log.Info(fmt.Sprintf("God bot epoch %d started", epoch))
			return
		}
		if err != nil {
			log.Error(fmt.Sprintf("Unable to get god node epoch to sync: %v", err))
		}
		time.Sleep(requestRetryDelay)
	}
}

func (process *Process) switchOnlineState(u *user.User, nextValidationTime time.Time) {
	testIndex := process.getCurrentTestIndex()
	onlines := process.sc.EpochNodeOnlines[testIndex]
	becomeOnline := pos(onlines, u.Index) != -1
	if becomeOnline {
		if attempts, err := process.tryToSwitchOnlineState(u, nextValidationTime, true); err != nil {
			process.handleError(err, fmt.Sprintf("%v unable to become online, attempts: %d", u.GetInfo(), attempts))
		}
	}
	offlines := process.sc.EpochNodeOfflines[testIndex]
	becomeOffline := pos(offlines, u.Index) != -1
	if becomeOffline {
		if attempts, err := process.tryToSwitchOnlineState(u, nextValidationTime, false); err != nil {
			process.handleError(err, fmt.Sprintf("%v unable to become offline, attempts: %d", u.GetInfo(), attempts))
		}
	}

	// Become online by default if state is newbie
	if !becomeOnline && !becomeOffline && !u.SentAutoOnline {
		identity := process.getIdentity(u)
		if identity.State == newbie || identity.State == verified {
			if attempts, err := process.tryToSwitchOnlineState(u, nextValidationTime, true); err != nil {
				log.Warn(fmt.Sprintf("%v unable to become online, attempts: %d, error: %v", u.GetInfo(), attempts, err))
			} else {
				u.SentAutoOnline = true
			}
		}
	}
}

func (process *Process) tryToSwitchOnlineState(u *user.User, nextValidationTime time.Time, online bool) (int, error) {
	var switchOnline func() (string, error)
	var stateName string
	if online {
		switchOnline = u.Client.BecomeOnline
		stateName = "online"
	} else {
		switchOnline = u.Client.BecomeOffline
		stateName = "offline"
	}
	// Try to switch online state till (nextValidationTime - 3 minutes) to leave time for submitting flips
	deadline := nextValidationTime.Add(-time.Minute * 3)
	attempts := 0
	for {
		hash, err := switchOnline()
		attempts++
		if err == nil {
			log.Info(fmt.Sprintf("%v sent request to become %s, tx: %s, attempts: %d", u.GetInfo(), stateName, hash, attempts))
			return attempts, nil
		}
		if time.Now().After(deadline) {
			return attempts, err
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
	state.availableFlips = identity.AvailableFlips
}

func (process *Process) passVerification(u *user.User) {

	log.Debug(fmt.Sprintf("%v required flips: %d", u.GetInfo(), process.getRequiredFlipsCount(u)))

	process.getFlipHashes(u, true, 3)

	process.getFlips(u, true)

	process.submitAnswers(u, true)

	u.TestContext.ShortFlipHashes = nil

	waitForLongSession(u)

	process.getFlipHashes(u, false, 10)

	process.getFlips(u, false)

	delay := time.Second * time.Duration(rand.Float64()*process.ceremonyIntervals.LongSessionDuration/2)
	log.Debug(fmt.Sprintf("%v delay before submitting long answers: %v", u.GetInfo(), delay))
	time.Sleep(delay)

	process.submitAnswers(u, false)

	u.TestContext.LongFlipHashes = nil
}

func (process *Process) initTest(u *user.User) {
	if u.TestContext != nil {
		u.TestContext.ShortFlipHashes = nil
		u.TestContext.LongFlipHashes = nil
	}
	u.TestContext = &user.TestContext{
		TestStartTime: time.Now(),
	}
}

type submittedFlip struct {
	hash        string
	wordPairIdx uint8
	txHash      string
}

func (process *Process) submitFlips(u *user.User, godAddress string) {
	flipsToSubmit := process.getFlipsCountToSubmit(u, godAddress)
	if flipsToSubmit == 0 {
		return
	}
	var submittedFlips []submittedFlip
	for i := 0; i < flipsToSubmit; i++ {
		flipPrivateHex, flipPublicHex, err := generateFlip()
		if err != nil {
			process.handleError(err, "unable to generate hex")
		}
		if !process.fastNewbie && process.getCurrentTestIndex() == 0 && flipsToSubmit > 1 {
			_, err := u.Client.SubmitFlip(flipPrivateHex, flipPublicHex, 0)
			if err != nil {
				log.Warn(fmt.Sprintf("%v got submit flip request error: %v", u.GetInfo(), err))
				continue
			}
			log.Info(fmt.Sprintf("%v submitted flip", u.GetInfo()))
			submittedFlips = append(submittedFlips, submittedFlip{})
			time.Sleep(time.Millisecond * 100)
			continue
		}
		wordPairIdx := uint8(i)
		if process.flipsChan != nil {
			process.flipsChan <- 1
		}
		log.Info(fmt.Sprintf("%v start submitting flip", u.GetInfo()))
		flipCid, txHash := process.submitFlip(u, flipPrivateHex, flipPublicHex, wordPairIdx)
		flip := submittedFlip{
			hash:        flipCid,
			wordPairIdx: wordPairIdx,
			txHash:      txHash,
		}
		log.Info(fmt.Sprintf("%v submitted flip %v", u.GetInfo(), flip))
		if process.flipsChan != nil {
			<-process.flipsChan
		}
		submittedFlips = append(submittedFlips, flip)
	}
	log.Info(fmt.Sprintf("%v submitted %v flips: %v", u.GetInfo(), len(submittedFlips), submittedFlips))
}

func (process *Process) submitFlip(u *user.User, privateHex, publicHex string, wordPairIdx uint8) (flipCid, txHash string) {
	submittedFlips := process.getIdentity(u).Flips
	resp, err := u.Client.SubmitFlip(privateHex, publicHex, wordPairIdx)
	if err != nil {
		log.Warn(fmt.Sprintf("%v got submit flip request error: %v", u.GetInfo(), err))
	}
	log.Info(fmt.Sprintf("%v start waiting for mined flip (resp: %v)", u.GetInfo(), resp))
	for {
		time.Sleep(requestRetryDelay)
		newSubmittedFlips := process.getIdentity(u).Flips
		if len(newSubmittedFlips) == len(submittedFlips)+1 {
			prevSubmittedFlips := mapset.NewSet()
			for _, flipCid := range submittedFlips {
				prevSubmittedFlips.Add(flipCid)
			}
			for _, flipCid := range newSubmittedFlips {
				if !prevSubmittedFlips.Contains(flipCid) {
					return flipCid, resp.TxHash
				}
			}
		}
	}
}

func determineFlipAnswer(flipHash client.FlipHashesResponse) byte {
	if !flipHash.Ready {
		return common.None
	}
	var answer byte
	bytes := []byte(flipHash.Hash)
	if bytes[len(bytes)-2]%2 == 0 {
		answer = common.Left
	} else {
		answer = common.Right
	}
	return answer
}

func (process *Process) getFlipsCountToSubmit(u *user.User, godAddress string) int {
	requiredFlipsCount := process.getRequiredFlipsCount(u)
	if process.getCurrentTestIndex() == 0 && u.Address == godAddress && requiredFlipsCount == 0 {
		requiredFlipsCount = initialRequiredFlips
	}

	flipsCountToSubmit := requiredFlipsCount
	userCeremony := process.getScUserCeremony(u)
	if userCeremony != nil {
		flipsCountToSubmit = userCeremony.SubmitFlips
	}

	log.Info(fmt.Sprintf("%v required flips: %d, flips to submit: %d", u.GetInfo(), requiredFlipsCount, flipsCountToSubmit))

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
	flipsCount := identity.RequiredFlips
	additional := identity.AvailableFlips - identity.RequiredFlips
	if additional > 0 {
		a := []byte(u.Address)
		for i := 0; i < additional; i++ {
			if a[len(a)-1-i]%2 == 0 {
				flipsCount++
			}
		}
	}
	return flipsCount - len(identity.Flips)
}

func generateFlip() (privateHex, publicHex string, err error) {
	var privateBytes []byte
	if privateBytes, err = randomBytes(rand.Int()%80000 + 80000); err != nil {
		return
	}
	privateHex = toHex(privateBytes)
	publicHex = toHex(reverse(privateBytes))
	return
}

func reverse(bytes []byte) []byte {
	var res []byte
	for i := len(bytes) - 1; i >= 0; i-- {
		res = append(res, bytes[i])
	}
	return res
}

func randomBytes(n int) ([]byte, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return nil, err
	}
	return bytes, nil
}

func toHex(bytes []byte) string {
	return "0x" + hex.EncodeToString(bytes)
}

func (process *Process) assertFlip(u *user.User, flipHash string, flip client.FlipResponse) {
	if toHex(reverse(common2.FromHex(flip.PrivateHex))) == flip.PublicHex {
		return
	}
	message :=
		fmt.Sprintf("%v private flip hex must be equal to reversed public one, cid %s", u.GetInfo(), flipHash)
	process.handleError(errors.New(message), "")
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

func (process *Process) getFlipHashes(u *user.User, isShort bool, allowableNotReadyCount int) {
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
		}
		if time.Now().After(deadline) {
			if err := process.checkFlipHashes(flipHashes, allowableNotReadyCount); err == nil ||
				errors.Cause(err) == notReadyFlipsAllowableCount {
				if err != nil {
					log.Warn(fmt.Sprintf("%v %v", u.GetInfo(), err))
				}
				break
			}
			process.handleError(err, fmt.Sprintf("%v didn't manage to get all flips ready: %v", u.GetInfo(), err))
		}
		if err = process.checkFlipHashes(flipHashes, 0); err == nil {
			break
		}
		log.Warn(fmt.Sprintf("%v unable to get %s flip hashes: %v", u.GetInfo(), name, err))
		time.Sleep(requestRetryDelay)
	}
	setFunc(flipHashes)
	log.Info(fmt.Sprintf("%v got %d %s flip hashes", u.GetInfo(), len(flipHashes), name))
}

var notReadyFlipsAllowableCount = errors.New("there is not ready flips")

func (process *Process) checkFlipHashes(flipHashes []client.FlipHashesResponse, allowableNotReadyCount int) error {
	if len(flipHashes) == 0 {
		return errors.New("empty flip hashes")
	}
	var notReadyFlips []string
	for _, f := range flipHashes {
		if !f.Ready {
			notReadyFlips = append(notReadyFlips, f.Hash)
		}
	}
	if len(notReadyFlips) > 0 {
		msg := fmt.Sprintf("Not ready flips: %v, allowableNotReadyCount: %v", notReadyFlips, allowableNotReadyCount)
		if len(notReadyFlips) > allowableNotReadyCount {
			return errors.New(msg)
		}
		return errors.Wrap(notReadyFlipsAllowableCount, msg)
	}
	return nil
}

func (process *Process) getFlips(u *user.User, isShort bool) {
	name := getSessionName(isShort)
	var flipHashes []client.FlipHashesResponse
	if isShort {
		flipHashes = u.TestContext.ShortFlipHashes
	} else {
		flipHashes = u.TestContext.LongFlipHashes
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
		process.assertFlip(u, h.Hash, flipResponse)
		flips = append(flips, flipResponse)
	}
	log.Info(fmt.Sprintf("%v got %v %s flips", u.GetInfo(), len(flips), name))
	return
}

func emptyFlip() client.FlipResponse {
	return client.FlipResponse{}
}

func (process *Process) submitAnswers(u *user.User, isShort bool) {
	var submitFunc func([]client.FlipAnswer) (client.SubmitAnswersResponse, error)
	name := getSessionName(isShort)
	log.Trace(fmt.Sprintf("%v start submitting %s answers", u.GetInfo(), name))
	var flipHashes []client.FlipHashesResponse
	if isShort {
		submitFunc = u.Client.SubmitShortAnswers
		flipHashes = u.TestContext.ShortFlipHashes
	} else {
		submitFunc = u.Client.SubmitLongAnswers
		flipHashes = u.TestContext.LongFlipHashes
	}
	allAnswers := process.getAnswers(u, isShort)
	var answers []client.FlipAnswer
	allowableExtraFlips := 0
	for i, answer := range allAnswers {
		if i >= len(flipHashes) {
			break
		}
		if flipHashes[i].Extra {
			if allowableExtraFlips == 0 {
				continue
			}
			allowableExtraFlips--
		}
		answers = append(answers, answer)
		if !flipHashes[i].Ready {
			allowableExtraFlips++
		}
	}
	resp, err := submitFunc(answers)
	if err != nil {
		log.Warn(fmt.Sprintf("%v unable to submit %s answers: %v", u.GetInfo(), name, err))
	} else {
		log.Info(fmt.Sprintf("%v submitted %d %s answers: %v, tx: %s", u.GetInfo(), len(answers), name, answers, resp.TxHash))
	}
}

func (process *Process) getAnswers(u *user.User, isShort bool) []client.FlipAnswer {
	var flipHashes []client.FlipHashesResponse
	if isShort {
		flipHashes = u.TestContext.ShortFlipHashes
	} else {
		flipHashes = u.TestContext.LongFlipHashes
	}
	var answers []client.FlipAnswer
	for _, flipHash := range flipHashes {
		answers = append(answers, client.FlipAnswer{
			WrongWords: false,
			Answer:     determineFlipAnswer(flipHash),
			Hash:       flipHash.Hash,
		})
	}
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
