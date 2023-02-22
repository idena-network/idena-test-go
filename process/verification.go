package process

import (
	"encoding/hex"
	"fmt"
	mapset "github.com/deckarep/golang-set"
	"github.com/idena-network/idena-go/api"
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

	process.initCeremonyIntervals()
	timeout := process.getTestTimeout()
	process.wg = &sync.WaitGroup{}
	epochNewUsers := process.sc.EpochNewUsersAfterFlips[process.getCurrentTestIndex()]
	newUsers := 0
	for _, epochInviterNewUsers := range epochNewUsers {
		newUsers += epochInviterNewUsers.Count
	}
	process.wg.Add(len(process.users) + newUsers)
	process.es = &epochState{
		userStates: make(map[int]*userEpochState),
		wordsByCid: &sync.Map{},
	}
	process.testUsers(process.users)

	process.startEpochBackgroundProcess(process.wg, timeout)

	ok := common.WaitWithTimeout(process.wg, timeout)
	if !ok {
		var nodeNames []string
		for _, u := range process.users {
			if u.IsTestRun() {
				nodeNames = append(nodeNames, u.GetInfo())
			}
		}
		process.handleError(errors.New("verification sessions timeout"), strings.Join(nodeNames, ","))
	}
	process.assert(process.getCurrentTestIndex(), *process.es)
	log.Info(fmt.Sprintf("************** All verification sessions completed (test #%d) **************", process.getCurrentTestIndex()))
}

func (process *Process) testUsers(users []user.User) {
	mutex := sync.Mutex{}
	for _, u := range users {
		go func(u user.User) {
			ues := &userEpochState{}
			process.testUser(u, process.godAddress, ues)
			mutex.Lock()
			process.es.userStates[u.GetIndex()] = ues
			mutex.Unlock()
			process.wg.Done()
		}(u)
	}
}

func (process *Process) initCeremonyIntervals() {
	u := process.getActiveUsers()[0]
	intervals, err := u.CeremonyIntervals()
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
		time.Minute*time.Duration(process.validationTimeoutExtraMinutes)
	log.Debug(fmt.Sprintf("Verification session waiting timeout: %v", testTimeout))
	return testTimeout
}

func (process *Process) updateNode(u user.User) {
	epoch := process.getCurrentTestIndex()
	nodeUpdates, ok := process.sc.EpochNodeUpdates[epoch]
	if !ok {
		return
	}
	nodeUpdate, ok := nodeUpdates[u.GetIndex()]
	if !ok {
		return
	}
	delay := nodeUpdate.Delay
	time.Sleep(delay)
	if epoch != process.getCurrentTestIndex() {
		log.Warn(fmt.Sprintf("%s unable to update node due to old epoch", u.GetInfo()))
		return
	}
	log.Info(fmt.Sprintf("%s stopping node to update", u.GetInfo()))
	process.stopNode(u)
	u.SetNodeExecCommandName(nodeUpdate.Command)
	process.startNode(u, node.DeleteNothing)
	log.Info(fmt.Sprintf("%s started updated node", u.GetInfo()))
}

func (process *Process) testUser(u user.User, godAddress string, state *userEpochState) {
	u.SetIsTestRun(true)
	defer func() {
		u.SetIsTestRun(false)
	}()
	process.initTest(u)
	go process.updateNode(u)
	go process.delegate(u)
	go process.undelegate(u)
	go process.killDelegators(u)
	go process.sendStoreToIpfsTxs(u)
	go process.kill(u)
	go process.addStake(u)
	go process.killInvitees(u)

	wasActive := u.IsActive()

	if !wasActive {
		process.switchNodeIfNeeded(u)
	}

	if !u.IsActive() {
		log.Info(fmt.Sprintf(skipSessionMessageFormat, u.GetInfo()))
		return
	}

	epoch := process.getEpoch(u)
	log.Info(fmt.Sprintf("%s epoch: %d, next validation time: %v", u.GetInfo(), epoch.Epoch, epoch.NextValidation))

	if !process.validationOnly {
		process.switchOnlineState(u, epoch.NextValidation)

		multiBotDelegatee := u.GetMultiBotDelegatee()
		if multiBotDelegatee != nil {
			go process.delegateTo(u, *multiBotDelegatee)
			u.SetMultiBotDelegatee(nil)
		}
	}

	if wasActive {
		process.switchNodeIfNeeded(u)
	}

	if !u.IsActive() {
		log.Info(fmt.Sprintf(skipSessionMessageFormat, u.GetInfo()))
		return
	}

	process.submitFlips(u, godAddress)

	process.provideDelayedFlipKeyIfNeeded(u, epoch.NextValidation)

	epochNewUsers := process.sc.EpochNewUsersAfterFlips[process.getCurrentTestIndex()]
	if len(epochNewUsers) > 0 {
		for _, epochInviterNewUsers := range epochNewUsers {
			if epochInviterNewUsers.Inviter == nil && u.GetIndex() == 0 || epochInviterNewUsers.Inviter != nil && *epochInviterNewUsers.Inviter == u.GetIndex() {
				process.createEpochInviterNewUsers(epochInviterNewUsers)
				process.testUsers(process.users[len(process.users)-epochInviterNewUsers.Count:])
			}
		}
	}

	waitForFlipLottery(u)

	process.passVerification(u, epoch.NextValidation, epoch.NextValidation.Add(time.Second*time.Duration(process.ceremonyIntervals.ShortSessionDuration)))

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

func (process *Process) switchOnlineState(u user.User, nextValidationTime time.Time) {
	testIndex := process.getCurrentTestIndex()
	onlines := process.sc.EpochNodeOnlines[testIndex]
	becomeOnline := pos(onlines, u.GetIndex()) != -1
	if becomeOnline {
		if attempts, err := process.tryToSwitchOnlineState(u, nextValidationTime, true); err != nil {
			process.handleError(err, fmt.Sprintf("%v unable to become online, attempts: %d", u.GetInfo(), attempts))
		}
	}
	offlines := process.sc.EpochNodeOfflines[testIndex]
	becomeOffline := pos(offlines, u.GetIndex()) != -1
	if becomeOffline {
		if attempts, err := process.tryToSwitchOnlineState(u, nextValidationTime, false); err != nil {
			process.handleError(err, fmt.Sprintf("%v unable to become offline, attempts: %d", u.GetInfo(), attempts))
		}
	}

	if !becomeOnline && !becomeOffline && !u.GetAutoOnlineSent() {
		identity := process.getIdentity(u)
		if identity.State == verified || identity.State == human || identity.State == newbie {
			if attempts, err := process.tryToSwitchOnlineState(u, nextValidationTime, true); err != nil {
				log.Warn(fmt.Sprintf("%v unable to become online, attempts: %d, error: %v", u.GetInfo(), attempts, err))
			} else {
				u.SetAutoOnlineSent(true)
			}
		}
	}
}

func (process *Process) tryToSwitchOnlineState(u user.User, nextValidationTime time.Time, online bool) (int, error) {
	var switchOnline func() (string, error)
	var stateName string
	if online {
		switchOnline = u.BecomeOnline
		stateName = "online"
	} else {
		switchOnline = u.BecomeOffline
		stateName = "offline"
	}
	// Try to switch online state till (nextValidationTime - 3 minutes) to leave time for submitting flips
	deadline := nextValidationTime.Add(-time.Minute * 5)
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

func (process *Process) collectUserEpochState(u user.User, state *userEpochState) {
	identity := process.getIdentity(u)
	state.madeFlips = len(identity.Flips)
	state.requiredFlips = int(identity.RequiredFlips)
	state.availableFlips = int(identity.AvailableFlips)
}

func (process *Process) passVerification(u user.User, shortStartTime, shortFinishTime time.Time) {

	publicFlipKeysChan := time.After(shortStartTime.Sub(time.Now()))

	process.sendPrivateFlipKeysPackages(u)

	log.Info(fmt.Sprintf("%v start waiting for %v", u.GetInfo(), shortStartTime))

	<-publicFlipKeysChan

	process.sendPublicFlipKey(u)

	waitForShortSession(u)

	userCeremony := process.getScUserCeremony(u)
	skipValidation := userCeremony != nil && userCeremony.SkipValidation
	if skipValidation {
		log.Info(fmt.Sprintf("%v skip validation", u.GetInfo()))
		return
	}

	requiredFlips, _ := process.getRequiredFlipsInfo(u)
	log.Debug(fmt.Sprintf("%v required flips: %d", u.GetInfo(), requiredFlips))

	process.getFlipHashes(u, true, 3)

	process.getFlips(u, true)

	time.Sleep(shortFinishTime.Sub(time.Now()) - time.Second*20)

	process.submitAnswers(u, true)

	u.GetTestContext().ShortFlipHashes = nil

	waitForLongSession(u)

	process.submitOpenShortAnswers(u)

	delay := time.Second * time.Duration(rand.Float64()*process.ceremonyIntervals.LongSessionDuration/2)
	log.Debug(fmt.Sprintf("%v delay before submitting long answers: %v", u.GetInfo(), delay))
	delayChan := time.After(delay)

	process.getFlipHashes(u, false, 10)

	process.getFlips(u, false)

	<-delayChan

	process.submitAnswers(u, false)

	u.GetTestContext().LongFlipHashes = nil
}

func (process *Process) initTest(u user.User) {
	if u.GetTestContext() != nil {
		u.GetTestContext().ShortFlipHashes = nil
		u.GetTestContext().LongFlipHashes = nil
	}
	u.SetTestContext(&user.TestContext{
		TestStartTime: time.Now(),
		Epoch:         uint16(process.getCurrentTestIndex()),
	})
}

type submittedFlip struct {
	hash        string
	wordPairIdx uint8
	txHash      string
}

func (process *Process) submitFlips(u user.User, godAddress string) {
	flipsToSubmit, words := process.getFlipsInfoToSubmit(u, godAddress)
	if flipsToSubmit == 0 {
		return
	}
	var submittedFlips []submittedFlip
	for i := 0; i < flipsToSubmit; i++ {
		flipPrivateHex, flipPublicHex, err := generateFlip(process.minFlipSize, process.maxFlipSize)
		if err != nil {
			process.handleError(err, "unable to generate hex")
		}
		if !process.fastNewbie && !process.validationOnly && process.getCurrentTestIndex() == 0 && flipsToSubmit > 1 {
			_, err := u.SubmitFlip(flipPrivateHex, flipPublicHex, 0)
			if err != nil {
				log.Warn(fmt.Sprintf("%v got submit flip request error: %v", u.GetInfo(), err))
				continue
			}
			log.Info(fmt.Sprintf("%v submitted flip, priv size: %v, pub size: %v", u.GetInfo(),
				len(flipPrivateHex), len(flipPublicHex)))
			submittedFlips = append(submittedFlips, submittedFlip{})
			time.Sleep(time.Millisecond * 100)
			continue
		}
		wordPairIdx := uint8(i)
		if process.flipsChan != nil {
			process.flipsChan <- 1
		}
		log.Info(fmt.Sprintf("%v start submitting flip, priv size: %v, pub size: %v", u.GetInfo(),
			len(flipPrivateHex), len(flipPublicHex)))
		flipCid, txHash := process.submitFlip(u, flipPrivateHex, flipPublicHex, wordPairIdx)
		flip := submittedFlip{
			hash:        flipCid,
			wordPairIdx: wordPairIdx,
			txHash:      txHash,
		}
		log.Info(fmt.Sprintf("%v submitted flip %v", u.GetInfo(), flip))
		if process.getCurrentTestIndex() > 0 {
			process.es.wordsByCid.Store(flipCid, [2]uint32{words[wordPairIdx].Words[0].Id, words[wordPairIdx].Words[1].Id})
		}
		if process.flipsChan != nil {
			<-process.flipsChan
		}
		submittedFlips = append(submittedFlips, flip)
	}
	log.Info(fmt.Sprintf("%v submitted %v flips: %v", u.GetInfo(), len(submittedFlips), submittedFlips))
}

func (process *Process) submitFlip(u user.User, privateHex, publicHex string, wordPairIdx uint8) (flipCid, txHash string) {
	submittedFlips := process.getIdentity(u).Flips
	resp, err := u.SubmitFlip(privateHex, publicHex, wordPairIdx)
	if err != nil {
		log.Warn(fmt.Sprintf("%v got submit flip request error: %v", u.GetInfo(), err))
	}
	log.Info(fmt.Sprintf("%v start waiting for mined flip (resp: %v)", u.GetInfo(), resp))
	if process.validationOnly {
		return resp.Hash, resp.TxHash
	}
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

func reverseAnswer(answer byte) byte {
	switch answer {
	case common.Left:
		return common.Right
	case common.Right:
		return common.Left
	default:
		return answer
	}
}

func (process *Process) getFlipsInfoToSubmit(u user.User, godAddress string) (int, []user.FlipWords) {
	requiredFlipsCount, words := process.getRequiredFlipsInfo(u)
	if process.getCurrentTestIndex() == 0 && u.GetAddress() == godAddress && requiredFlipsCount == 0 {
		requiredFlipsCount = initialRequiredFlips
	}

	flipsCountToSubmit := requiredFlipsCount
	userCeremony := process.getScUserCeremony(u)
	if userCeremony != nil && userCeremony.SubmitFlips != nil {
		flipsCountToSubmit = *userCeremony.SubmitFlips
	}

	log.Info(fmt.Sprintf("%v required flips: %d, flips to submit: %d, words: %v", u.GetInfo(), requiredFlipsCount, flipsCountToSubmit, words))

	return flipsCountToSubmit, words
}

func (process *Process) getScUserCeremony(u user.User) *scenario.UserCeremony {
	ceremony := process.sc.Ceremonies[process.getCurrentTestIndex()]
	if ceremony == nil {
		return nil
	}
	return ceremony.UserCeremonies[u.GetIndex()]
}

func (process *Process) getRequiredFlipsInfo(u user.User) (int, []user.FlipWords) {
	flipsToSubmit, words, err := u.GetRequiredFlipsInfo()
	process.handleError(err, fmt.Sprintf("%v unable to get required flips info", u.GetInfo()))
	return flipsToSubmit, words
}

func generateFlip(minSize, maxSize int) (privateHex, publicHex string, err error) {
	var privateBytes []byte
	minSize /= 2
	maxSize /= 2
	if privateBytes, err = randomBytes(rand.Int()%(maxSize-minSize) + minSize); err != nil {
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

func (process *Process) assertFlip(u user.User, flipHash string, flip client.FlipResponse) {
	if toHex(reverse(common2.FromHex(flip.PrivateHex))) == flip.PublicHex {
		return
	}
	message :=
		fmt.Sprintf("%v private flip hex must be equal to reversed public one, cid %s", u.GetInfo(), flipHash)
	process.handleError(errors.New(message), "")
}

func (process *Process) assertFlipWords(u user.User, flipHash string, flipWordsResponse api.FlipWordsResponse) {
	if flipWordsResponse.Words[0] == flipWordsResponse.Words[1] {
		message :=
			fmt.Sprintf("%v equal flip words: %v, cid %s", u.GetInfo(), flipWordsResponse.Words, flipHash)
		process.handleError(errors.New(message), "")
	}
	if words, ok := process.es.wordsByCid.Load(flipHash); ok {
		if uint32(flipWordsResponse.Words[0]) != words.([2]uint32)[0] || uint32(flipWordsResponse.Words[1]) != words.([2]uint32)[1] {
			message :=
				fmt.Sprintf("%v invalid flip words: %v, expected: %v, cid %s", u.GetInfo(), flipWordsResponse.Words, words, flipHash)
			process.handleError(errors.New(message), "")
		}
	}
}

func waitForFlipLottery(u user.User) {
	waitForPeriod(u, periodFlipLottery)
}

func waitForShortSession(u user.User) {
	waitForPeriod(u, periodShortSession)
}

func waitForPeriod(u user.User, period string) {
	log.Info(fmt.Sprintf("%v start waiting for period %v", u.GetInfo(), period))
	for {
		epoch, _ := u.GetEpoch()
		currentPeriod := epoch.CurrentPeriod
		log.Debug(fmt.Sprintf("%v current period: %v", u.GetInfo(), currentPeriod))
		if currentPeriod == period {
			break
		}
		time.Sleep(requestRetryDelay)
	}
	log.Info(fmt.Sprintf("%v period %v started", u.GetInfo(), period))
}

func waitForLongSession(u user.User) {
	waitForPeriod(u, periodLongSession)
}

func waitForSessionFinish(u user.User) {
	waitForPeriod(u, periodNone)
}

func (process *Process) getCurrentTestIndex() int {
	return process.testCounter
}

func (process *Process) getFlipHashes(u user.User, isShort bool, allowableNotReadyCount int) {
	name := getSessionName(isShort)
	var loadFunc func() ([]client.FlipHashesResponse, error)
	var setFunc func([]client.FlipHashesResponse)
	if isShort {
		loadFunc = u.GetShortFlipHashes
		setFunc = func(flipHashes []client.FlipHashesResponse) {
			u.GetTestContext().ShortFlipHashes = flipHashes
		}
	} else {
		loadFunc = u.GetLongFlipHashes
		setFunc = func(flipHashes []client.FlipHashesResponse) {
			u.GetTestContext().LongFlipHashes = flipHashes
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
			if err := process.checkFlipHashes(u, flipHashes, allowableNotReadyCount, isShort); err == nil ||
				errors.Cause(err) == notReadyFlipsAllowableCount {
				if err != nil {
					log.Warn(fmt.Sprintf("%v %v", u.GetInfo(), err))
				}
				break
			}
			process.handleError(err, fmt.Sprintf("%v didn't manage to get all flips ready: %v", u.GetInfo(), err))
		}
		if err = process.checkFlipHashes(u, flipHashes, 0, isShort); err == nil {
			break
		}
		log.Warn(fmt.Sprintf("%v unable to get %s flip hashes: %v", u.GetInfo(), name, err))
		time.Sleep(requestRetryDelay)
	}
	setFunc(flipHashes)
	log.Info(fmt.Sprintf("%v got %d %s flip hashes", u.GetInfo(), len(flipHashes), name))
}

var notReadyFlipsAllowableCount = errors.New("there is not ready flips")

func (process *Process) checkFlipHashes(u user.User, flipHashes []client.FlipHashesResponse, allowableNotReadyCount int, isShort bool) error {
	if len(flipHashes) == 0 {
		return errors.New("empty flip hashes")
	}
	var notReadyFlips []string
	var flipsWithoutWords []string
	for _, f := range flipHashes {
		if !f.Ready {
			notReadyFlips = append(notReadyFlips, f.Hash)
			continue
		}
		if !isShort && process.getCurrentTestIndex() > 0 {
			flipWordsResponse, err := u.GetFlipWords(f.Hash)
			if err != nil {
				flipsWithoutWords = append(flipsWithoutWords, f.Hash)
				log.Warn(fmt.Sprintf("%v unable to get flip words %s: %v", u.GetInfo(), f.Hash, err))
				continue
			}
			process.assertFlipWords(u, f.Hash, flipWordsResponse)
		}
	}
	notReadyFlipsCount := len(notReadyFlips) + len(flipsWithoutWords)
	if notReadyFlipsCount > 0 {
		msg := fmt.Sprintf("Not ready flips: %v, flips without words: %v, allowableNotReadyCount: %v", notReadyFlips, flipsWithoutWords, allowableNotReadyCount)
		if notReadyFlipsCount > allowableNotReadyCount {
			return errors.New(msg)
		}
		return errors.Wrap(notReadyFlipsAllowableCount, msg)
	}
	return nil
}

func (process *Process) getFlips(u user.User, isShort bool) {
	name := getSessionName(isShort)
	var flipHashes []client.FlipHashesResponse
	if isShort {
		flipHashes = u.GetTestContext().ShortFlipHashes
	} else {
		flipHashes = u.GetTestContext().LongFlipHashes
	}

	var flips []client.FlipResponse
	for _, h := range flipHashes {
		if !h.Ready {
			flips = append(flips, emptyFlip())
			continue
		}
		flipResponse, err := process.getFlip(u, h.Hash)
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

func (process *Process) getFlip(u user.User, hash string) (client.FlipResponse, error) {
	return u.GetFlip(process.decryptFlips, hash)
}

func emptyFlip() client.FlipResponse {
	return client.FlipResponse{}
}

func (process *Process) submitAnswers(u user.User, isShort bool) {
	var submitFunc func([]client.FlipAnswer) (client.SubmitAnswersResponse, error)
	name := getSessionName(isShort)
	log.Trace(fmt.Sprintf("%v start submitting %s answers", u.GetInfo(), name))
	var flipHashes []client.FlipHashesResponse
	if isShort {
		submitFunc = u.SubmitShortAnswers
		flipHashes = u.GetTestContext().ShortFlipHashes
	} else {
		submitFunc = u.SubmitLongAnswers
		flipHashes = u.GetTestContext().LongFlipHashes
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

func (process *Process) getAnswers(u user.User, isShort bool) []client.FlipAnswer {
	var flipHashes []client.FlipHashesResponse
	if isShort {
		flipHashes = u.GetTestContext().ShortFlipHashes
	} else {
		flipHashes = u.GetTestContext().LongFlipHashes
	}
	var answers []client.FlipAnswer

	var reportAll, noApproves, severalIncreasedGrades bool
	randomValue := rand.Intn(10)
	reportAll = randomValue == 1
	noApproves = randomValue == 2
	severalIncreasedGrades = randomValue == 3

	userCeremony := process.getScUserCeremony(u)
	failShortSession := userCeremony != nil && userCeremony.FailShortSession

	increasedGradeCnt := 0

	for _, flipHash := range flipHashes {
		var grade byte
		if reportAll {
			grade = 1
		} else {
			grade = byte(rand.Intn(6))
			if noApproves && grade >= 2 {
				grade = 0
			}
			if !severalIncreasedGrades && grade > 2 && increasedGradeCnt == 1 {
				grade = 2
			}
			if grade > 2 {
				increasedGradeCnt++
			}
		}
		answer := determineFlipAnswer(flipHash)
		if failShortSession && isShort {
			answer = reverseAnswer(answer)
		}
		answers = append(answers, client.FlipAnswer{
			Grade:  grade,
			Answer: answer,
			Hash:   flipHash.Hash,
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

func (process *Process) submitOpenShortAnswers(u user.User) {
	txHash, err := u.SubmitOpenShortAnswers()
	if err == nil && len(txHash) == 0 {
		return
	}
	if err != nil {
		log.Warn(fmt.Sprintf("%v unable to submit open short answers: %v", u.GetInfo(), err))
	} else {
		log.Info(fmt.Sprintf("%v submitted open short answers, tx: %s", u.GetInfo(), txHash))
	}
}

func (process *Process) provideDelayedFlipKeyIfNeeded(u user.User, nextValidationTime time.Time) {
	users, present := process.sc.EpochDelayedFlipKeys[process.getCurrentTestIndex()]
	if !present || pos(users, u.GetIndex()) == -1 {
		return
	}
	log.Info(fmt.Sprintf("%v providing delayed flip key", u.GetInfo()))
	sleepTime := nextValidationTime.Sub(time.Now()) + shortSessionFlipKeyDeadline + time.Second*5
	time.Sleep(time.Second * 20) // Time for mining last operations
	go process.stopNode(u)
	time.Sleep(sleepTime)
	process.startNode(u, node.DeleteNothing)
}

func (process *Process) getEpoch(u user.User) client.Epoch {
	epoch, err := u.GetEpoch()
	process.handleError(err, fmt.Sprintf("%v unable to get epoch", u.GetInfo()))
	return epoch
}

func (process *Process) sendPrivateFlipKeysPackages(u user.User) {
	for {
		if err := u.SendPrivateFlipKeysPackages(); err == nil {
			return
		} else {
			log.Warn(fmt.Sprintf("%s unable to send private flip keys package: %v", u.GetInfo(), err.Error()))
			time.Sleep(requestRetryDelay)
		}
	}
}

func (process *Process) sendPublicFlipKey(u user.User) {
	err := u.SendPublicFlipKey()
	process.handleError(err, fmt.Sprintf("%v unable to broadcast public flip key", u.GetInfo()))
}
