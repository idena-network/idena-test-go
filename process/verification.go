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
	process.testExternalUsers(process.externalUsers)

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

func (process *Process) testExternalUsers(users []user.User) {
	for _, u := range users {
		go process.testExternalUser(u)
	}
}

func (process *Process) testExternalUser(u user.User) {
	process.startNode(u, node.DeleteNothing)

	if len(u.GetAddress()) == 0 {
		process.getNodeAddresses([]user.User{u})
	}

	var period string
	for {
		flipsToSubmit, _ := process.getFlipsInfoToSubmit(u, "")
		period = process.getEpoch(u).CurrentPeriod
		if flipsToSubmit > 0 || period != periodNone {
			break
		}
		log.Info(fmt.Sprintf("%s no required flips to submit", u.GetInfo()))
		time.Sleep(requestRetryDelay)
	}
	if period == periodNone {
		process.submitExternalUserFlips(u, "")
		time.Sleep(time.Second * 30)
	} else {
		log.Info(fmt.Sprintf("%s validation started, flips not submitted", u.GetInfo()))
	}
	process.stopNode(u)
}

func (process *Process) submitExternalUserFlips(u user.User, godAddress string) {
	flipsToSubmit, _ := process.getFlipsInfoToSubmit(u, godAddress)
	if flipsToSubmit == 0 {
		return
	}
	var submittedFlips []submittedFlip
	for i := 0; i < flipsToSubmit; i++ {
		flipPrivateHex, flipPublicHex, err := generateFlip(process.minFlipSize, process.maxFlipSize)
		if err != nil {
			process.handleError(err, "unable to generate hex")
		}
		wordPairIdx := uint8(i)
		if process.flipsChan != nil {
			process.flipsChan <- 1
		}
		log.Info(fmt.Sprintf("%v start submitting flip, priv size: %v, pub size: %v", u.GetInfo(),
			len(flipPrivateHex), len(flipPublicHex)))
		flipCid, txHash := process.submitExternalUserFlip(u, flipPrivateHex, flipPublicHex, wordPairIdx)
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

func (process *Process) submitExternalUserFlip(u user.User, privateHex, publicHex string, wordPairIdx uint8) (flipCid, txHash string) {
	resp, err := u.SubmitFlip(privateHex, publicHex, wordPairIdx)
	if err != nil {
		log.Warn(fmt.Sprintf("%v got submit flip request error: %v", u.GetInfo(), err))
	}
	return resp.Hash, resp.TxHash
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

	process.submitFlips(u, godAddress)

	if wasActive {
		process.switchNodeIfNeeded(u)
	}

	if !u.IsActive() {
		log.Info(fmt.Sprintf(skipSessionMessageFormat, u.GetInfo()))
		return
	}

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
	// TODO
	return common.Right
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

const publicPart = "0xf9321cf93219b91cfeffd8ffe000104a46494600010100000100010000ffdb0084000d090a0b0a080d0b0a0b0e0e0d0f13201513121213271c1e17202e2931302e292d2c333a4a3e333646372c2d405741464c4e525352323e5a615a50604a51524f010e0e0e131113261515264f352d354f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4fffc000110800b400f003011100021101031101ffc401a20000010501010101010100000000000000000102030405060708090a0b100002010303020403050504040000017d01020300041105122131410613516107227114328191a1082342b1c11552d1f02433627282090a161718191a25262728292a3435363738393a434445464748494a535455565758595a636465666768696a737475767778797a838485868788898a92939495969798999aa2a3a4a5a6a7a8a9aab2b3b4b5b6b7b8b9bac2c3c4c5c6c7c8c9cad2d3d4d5d6d7d8d9dae1e2e3e4e5e6e7e8e9eaf1f2f3f4f5f6f7f8f9fa0100030101010101010101010000000000000102030405060708090a0b1100020102040403040705040400010277000102031104052131061241510761711322328108144291a1b1c109233352f0156272d10a162434e125f11718191a262728292a35363738393a434445464748494a535455565758595a636465666768696a737475767778797a82838485868788898a92939495969798999aa2a3a4a5a6a7a8a9aab2b3b4b5b6b7b8b9bac2c3c4c5c6c7c8c9cad2d3d4d5d6d7d8d9dae2e3e4e5e6e7e8e9eaf2f3f4f5f6f7f8f9faffda000c03010002110311003f00f4ea002800a002800a002800a002800a002800a002800a002800a00af79796f636ed3dd4ab1c6bd589a00e4ef7c76833fd9f64f28ce03b9207d718feb4ae3b1463f88178bb649ec236849e4ab1071d3f9d170b1d1691e2ed2f5474891da199ba24a319fa1e945c2c7400e69882800a002800a002800a002800a002800a002800a002800a0028002714011b4d1a8c97503dcd26ec09363c1046453016800a002800a002800a004271401e4be2ad72e353d5e648a379ad6dd8aa28e991dce2a6e524d9cf6cbfb972cde691e9ce052ba452849f41cf15e419255fcac7a1c509dc6e9c910a3852183b0c1e00eb5443d0f61f06ea326a3a22492b1678cec2c7a9c7ad08937e98050014005001400500140050014005001400500140050014019daf473cba35d476b218e5643861dbd693762a2aeec727a23db5bdb793b5d829cb316ebef5cce4dbbb3d195251564ce82db50558b7c60851ea69a9339e74fb9a96772977009633c1f7ade2ee8e694795d8b354485001400500140105ebb4569348b8dcb1b119fa500797f874ff00a1a965f9a4762c7d79ae69bb48eda505ca7451988a3064c13ebdc52b9a5adb155e28fcb6464f949f97838a46addce6bc45a75bc36a2e6150ac5b071d0d5d36db396ac558eabe194eab63736cf2299158305cf38c75add1c8ceea9882800a002800a002800a002800a002800a002800a002800a0042b9183d28038ebdb26d3f542100f2dd89507b83dbf0ae59ab33ba1539a1a86b16c6e7419a28c0f3080547e3571dd18c9dcbba1e74bb38e0e5c01f367d6b6b98346fc3711cc32879ee3b8a61626a041400500140115c279904883f8948a4c6b73cb8d94ab88623247b49e871deb9b9b5d4f4146f1d0b7a6b5cc25d2e26675519049e45293d46a0d21ad0de4ec5c5cbec27a6471fa552924887195c86fed8c9a6bc3b9d9fae5b9e69296ba0e51f7753a4f04582db46ee8a000bb59b1f78fd6b485db39ab25156475f5b1805001400500140050014005001400500140050014005001400500616b0ab2ea10293c229623ea6b1abab36a5a264528566110206464d4c56a396c38f0b900e0715d064221746255ba1e0f715360342defcfdd9ff000615422fa3abae54822810ea0028010d0071be2a47b3bbf32de3cf9a3271faff004ae6ab1d6e7661e7a5998b69f6d914cab66655208c82739fcaa544de53b01bab9b3c24d6ac8cc4e1739cd37104d3342c55afcac710d9249b4f3da9415d99d59595cededade3b785638d70a2ba92b1c326e5ab26a620a002800a002800a002800a002800a002800a002800a0028020ba9841034879c741ea681c5733b1c8e957124ef713ddc8c64f31b3bb8c0cf0315cd27ef1d5c9caac8dab083ed62533a32e40d9c7207ad690899d4f77607b7b9b627721953b32f6fad68ae8cb46343c6dc0619aa0e519f23ed743907b8e86901243249148763018ea334099a905cabf0ff002b7e868b08b140064500729e37bf862b248d1c19838381d40acaa496c75e16939339ab3d4d5504d6f3f9729cef072056699b3b3d24175a8237ccede74efd187414485756b451b3e1e645bd89c9c2f4c9fa510b5c8ad17ca76892c6e3e5753f435d1738ecd0fcd31050014005001400500140050014005001400500140050014019ba9ca032c5b0b6483ed9cf145cd69c6fa904f0dbb4c9be38da56f9b24e0802a795365a6ed724d29a55795276ddc82a73db9a689aa93d4d4c53312b4d610c84b05d8c79caf19a077652974fb95398e4571e8dc52b05c4586e901df1a803fda154803730257299c766068e641cac9a1bc68f08ccb267d1ba54f322bd9bb5c75edda2623320566e80753432a106f53cf3c468c9a9b20e12450cbfd6b96ac753d3a122082c6dae235321cb11ce38acd361515dec4dfd9f1c0aa234018f72726a9b638460b5b17953ecf1ed63f315e99a111269ec58f0e5ab5bead2ce9348e045bb63313839fd6b4a776c8ad2e68729da5add2cd0ab9c2b138c67bd741e74e3caec59cd3245a002800a002800a002800a002800a002800a00280109a00c0d5ef6281de4790aec236fbd44e563b30f4a53d11876577a9dedc4da95bc0aeccbb22566e8335116e5aa3baad3a34a2a9c99d569b6acb1c6f385f3719c8ed9eb5aa47955669bb4763449c0a66447e7478fbebf9d03b30321ec38f5a02c51beba8a3502550727039efd85234a74e52339efada43222bac6f8e01209fa54dd1d51a135676b905af99e73b46619d87f06e284ff003cd4a45545a59ab16edef1e6b4133db32c873b909193edd2b44612859d93d0ccbe89359b3322c0f14f6e701188248f4e2b397beac6aa2e8b5aee605b29b79df7a9f28f3cf6ae5b3b9d6df3c74dcbe248e470403c0e335464d4921c63df2e4f269d88e6d2c6bd85a8b6b396f5bab2ed18eca3ff00af5bd38d95cc1cb9a5ca5bb6b9b595de09655746c32e0e307d2ad314e12b5d22db6a515ac58958b6dea4f5fd69b69111a129bd0bb6d729731892224a9e9918a13329c1c1d993d3242800a002800a002800a002800a002800a0064a76a31f41402312eeca3b9189555a2623393fe149a4ceaa55650d84d3ed6df4e77485d9b27804e368f6a4b42eb549d5b3637ed97335fe6d8911026327191c77a1c9df417b28a86bb94eeb53d561884c5229e167daa1411b874a96e48de8e1e8cfddbd996adafbf742596dc40adcb2c8b8a6998d4a3695a2ee581ac5908252b228dbd55b8aae65633786a97d8c6d55a76b65292070589e074a867a18551e6b3472e049bc819dd9cd667b52e5e5d0ed34186de448a54237367209c1c8ab8ab9f3f8ba92bb5d0bb711335c3812a023eee319031dc77ad4e583f776292bf2df63dacecdb59ba007d79a837b5d7be56d6f4f48dd194e37f3f43deb3a91ea4d1a8ef632d2dcf52c78ebef59a474ca7a1655249644b787976ea7fbabdcd52577630babdd9d38023b554d81ca80bb148e6ba95b6396fef5ca36f649f6a951c13cee51c1e7fa54a46d3aaf955892ed20b99d6de687f748be63b11c16ec28766c54e5282e64f525675b4f2b61118181b739033da93b2255e77bea6bc6c194114d1cd6b0fa60140050014005001400500140050014011cc3744e08e08a06b7336d2fa1b8010655fa157183f85099b4e938ea4173688f2b363e7380bb71c7b9a4d150a964365db69146b6ea1db72ef53d949e68d87f1bd44f3fca7649c46acce560518185cf5feb4ae3e4babc7e651bc81ee35094bc819368fb87073e952d33a29cd420ac8cf12195e6b5981638f9377271f5ef52fb1d76b454d3376d9434491ca80ec1e59ee46475ab5b1e74dbe66d32187495676758400588ebce3d714b94d1e2da566c99216b5405a10af11243ae093ffebaab58c9cbda3df725b4bfb4d4a26741b268fef2bae1969a7726a519d2693d9911b16bd948ba465504ec2a400411d4d1ca57b554d6856369756d673dbdcb1997ac730e4fd0e7a5438591529c66d4a1a14c6d1183db1588db7d4b7616d3c0b349e56f9255fb80f38ffeb56f15644be576d4404ba11b4a8047ce4e195bd7e9ed42b972493dcd8486508bbca9948f9f6f018d696395c936ec5594c334124324d2390c18274c60ff002a9d0d1292b3486c96cd34eb70f72a91ae098b1d7db34ac52a8945c6c6a5acdba564ed8cad51cd25a1728202800a002800a002800a002800a00280108a00c8bfd3de5dc046b220e5467041fad4d8de9d4b751b6cce726686681d0606e39cfe3de9a639dba3b93c6d105f9092e7f888e6a934ccdea5796dd640f718533a82aa5bb0f4a4e37d8d6153974e863c367751cafb6112927790afdf359ae6476cab539ab5ec3ae2d6e62433dc5bf96236cc4473d7b1028689a7522df2c5ee536d46e21bd13481d17195407afd7d6a79ac757d5a33a6d23a182fbce8d66019636ea08f9bf2ad56a7952a5caf948eeecd776eb7dd1ccdf37a827e84d0d762e9cfa4b63023967bfbd323466df682933af7c7a8acb56cf4a5185286f7372e21bb4b70f0cafc7231cedf735a599e7a9c5cb54598ee95edd0794ee7186c907ebd69a6ec632a6d48e7eeeeed6cf549011ba05c481477cf6fcc5652b291d54a84eae85fd22ea4b890dc6ff2f70c797e8bd7f3ab8bb86229282e5b5cdc628d1ed550cc46791d6b4b1c31ba7a942d66d467958dd5b2c4233f29539273495cdea4694527077625fa7977704911f97243e075a97b8e93bc5a636467b8ba687037461581faf7a616e58dc9ac679639d564dcfb9be53b71b47bd25b915209aba3681cd51cc2d0014005001400500140050014005001400500181400d28a7aa8fca8013cb4ce76afe5400e0a0740050055d4d0bd84aa8016c719a4f62e9be5926ce62d34ef32f8c52302010496191f4acd4753d5a9885ecee8d5646b447dd3204fe16651f28ad0e152536b416d254b8b78d960f31d24ce0b72bef4214e2e32b5cbb1450c72348aa15a4ea074268b19394daf419700223796483df1daa870d5ea733ac4f288115418d9324b71f3fbe6b39367ab848479aeca516857b730a4eee8dbbe6dacd83ea054a837ab3678ca54e4d451bb6764902450e479fbb706651d3d2ad46c7055ace6dcba1b1b9208c6f2153a66aee7134e4f42a36ad6c2fc5a39525b8520f7f4a8e75b1bac2cf939c491a35b678c45e518db800e783d0d3146fcdb91e9c43b5c4ccc1d98eddf8c0c0a512ab6ca28bb1200e4292c1b919e45598b6ec68c672a09f4a4623a800a002800a002800a002800a002800a002800a002800a00280239c131301d681adccd58c472fda1815763b78e9f5a2d636e66d7285cdbef88ace77ee073c6011458233b3ba33ec647b1b89232cef1c87824702a763a6adaa453ea8b971a5417732b4acff260a156c01ed8aab5cc618874d59130bdb78d59589528db39145ec66e9cdbba285fd924c1240c6589ce485e99fc293d4de9d571bad8d0805acd1ab08412830430195fc299cd3e64f72b5ed9cd2ccbb113ca031b73d6934cde9ce318d9b2bcff00da014247170720e4f007e152ee690f677bb66469d636f72a93c53b2de46c772b1cf23be2a124ceeaf5a70d1af759b086678bcbb854f2cf571f2b71fecd6a79cd46f7895b51bd36b66c1204d990318e07e14a6f95686b86a5ed67ab29693a8dcb4a37b3150464eee133d062a6326ceac4d0825a1d9db49e6264e3f0ad0f1651b326a04140050014005001400500140050014005001400500140050047300626073c8a0119f717060b212ac425c638ce29bd8da9c39a56b91cc2692ea10ad8831ca75ddf8d4dcb8f2c62d752bcb24913cab34ab146640b0e1724fad2ea5a8a6938ea4e25f2ca82d90dfc287249f5cd333e5bea914e683ecf2059d8b93cab86fbfec47ad268da32babc4af12cf1cccf1c72c499ce471c7b8eff00954ea692e471d771f7cb745daf6cae4c72a8198b03047bfad31d170f8271d3b928bd9afa2da7f7240004c4e0127b814eeccdd28d395d6a6842c91ed0f2867231bbfbdef548e69a6ddd2209aded2290dfac4a0b9c3ba820e3d69595ee5aa9392e4b99fac6a2209bcb50acc1728fdc7ad44a563a6850728735c874dbb4b9b693cf5db1eef97cc1d5b1da9a7709c1c1ae564b67606dcce634605be62480471472db61d5afcf6b9bda510d079808c39dd81daad1c357e22fd0641400500140050014005001400500140050014005001400500437448b772b8ce38cd038dafa9930b79f01b7b98d4bed3bb6f4fc28bdce86b925cd164161725d40554c4448c8ec2a14b536ab4acf57ab22d3af06a978dfb90b1db125475dcdeb4295d955a8fb086fab27bedd209224c6170c71c1fcea99952b27763fcedd2811c31cb2ed0086931c7d0d0c3974bdc7cf3422032c70a650fcd9ed8a098c257b3664dd69e6fef84e2558e12a01507a7ae31516d4eca75d52872daecd616f0b44a89007f287eec93c67d2af47a1c4ea4af7bee366f366f2d6308a54e0b63ee7d3140e368dee4ba8dc7d9a000920f63eb4a4ec85469f3c8e2754bb3777aa634f51c7726b16eecf7234bd9d1772e47772a5cc768722da35dcc7691d3b7d69b9bb9c6a1171badcd8b39ee2eee564752d1918c74280f7ad63dce7a918c236ea6e58e1494000007e3f8d51c33d752f0a080a002800a002800a002800a002800a002800a002800a0028020bc3b6dd8ed0dd38a071dce76fece492368ede5682207e6543f7bdf352cf428d6507792bb1d65a0bc70347f6b6d920e540fe75318155b19193bf29a361669636eebe66ee7e63b40c568a291c95aafb47723bf89f6e57e63d5589c73ee6863a324b7391d50ddcb7e58ee463fed7ddfa1ac64ddcf730ca9aa7dcbd6139dff623e63a4bf79ce48c814d37b1cb5e09fbfd8e8ac6cc456c8b30048ce02f400d6a91e655abcd2ba1ab1e2f776f902e482a8415fc7d29751dfdcb097774b1a131aed6079dc001f9d0d853a77dccdbabb99a291a789648b3c87e0ad436fa9d54a9c54bdd7664496f6cd6ef28b76f90023700463ea28491552a4d3e56ca3631432457124be716760bba361c03db07ad2b263e6941a68e8a3b33636062b25134e171f39c67fa0ad146c8e395455277a9b16ec1a7548cddaa452b6432e739f4c1a0ceaa8dfdc7a1a429988b40050014005001400500140050014005001400500140050057be19b66f4e33f4a0a8ee61c176d1ce2361f26485cb0e9fccd46a75ca09ad372c0d4f0dfba45909eb863fcf155cc43a1a7bda13b5cc734798dd43f420f6fad3b99a838b2c90762e42b0c722999eeca1269f6d70d26e8579c1ea6a796e742af382dcaf736d6f6cb0a95452ce06f07a0a1c6cca55672bd8d3691204084eedbe8dcd55ec8c14652d4ae2f2132a9124786e09cf20d4a91a3a52b6c4d246b3c647c920cf233d455111bc4a571a6acd18092b08c7fcb33fe352e26b0afcaef61d0592429e5be3c931b6e5dc4e78fa0a564853aae6ee723a14b7777a8ccb0b948109223c70c45631bb91e9d68c614d36b53b6b5936db872bcf7dc467e95d173c89c6ecaf1ea5bb5516ceb91b776783b0d4f31aca85a9731b8b54720b40050014005001400500140050014005001400500140050056be655b57672428e49141514dbd0c19af21b85709745805c0dca319f4a86d1db1a338ee88eda29d37bc3e5ba1ea51b907d7079a45d4717652274b79da685dc127180e472bebd2aec66ea4752ecacd6b6b2349707d4315271f853d8c1253968875abab6672cb86180f9c03f85170a8ada187e27d4a05b476491435b9ddea49c714a4d5cda9d39462d96668bfb4f4f82e6ddf6f9918638f7e6896a830f5791ea73f234b1dd8588b10bf864d657699ec479654eece9ad4cb12a3ca49cae721718fa8ad533c7a9cadbb17a5b81f6179778185272bfce9b6634e0dcd44c8d1ae1ae0a49248e43e42a93dba126a13b9d3888286890eb4d2c58cd25bac4029cb79b924b027d29a8a482588e749b6496d1bac0c666f2d412738ceefc295adb8a6d392e543ad6c6da68bcfb49dc4a09c3fafb1cf6a689a956a27cb25a1d045f7072381daa8e263e800a002800a002800a002800a002800a002800a002800a00caf13175d02eda338654ce7f1a996c74e0ededa373cda0927967450ed963eb5cc9b67d44a3151bb3b5d16cde161b98ee033f5ade08f07195a32d117ee6695aecc3147fc3f3b13dbdbd6b46ce5845285db248f7a9dac461b90cc79cfa62910ecf531b5bd463b3db085124afcb007000a894ac7761b0eeaeaf446378ca18a2d092e10a933155e0f3cf273ebd29d93d4caa55714e0cd7f075dc6fe19b70ad831e51c1f5078e7e98aa39ecdb3565861da4b49112c72d94df45916a52bd884e27c340c4843b1b6f1c7ae0d0cbd63b92dc441b4e74b74cee5c0e4024516d08849f3dd8c8ade289ff768aa620140efc8ef4688739ca5bb356e2069a01b6431b8e8c067fc8a0e784b95ea73716a170b7021bbb9842484a82898c11d8e6927aea7a2e8c1c6f05a9a71c64db9554de01f9792a453672b6b9bde34ec03adba897687ef8a1184da72d0b34c80a002800a002800a002800a002800a002800a002800a00a9aa41f69d36e21fefc647e949ec694a5cb34ce12cf446de8508320e40271bab08c4f7aa635729d64056da45265223da1595881b0f6ad96878d27cfb2277b78e40aee58f3b81cfafd2a9d999a934ec67dd48925d794a84cea0619ce173da933a209c6376f4338c7756172ef7d6df698e50034800383ebec2b369dceb5285585a0ecd18be3c8521b2b329808ccc703a74ffebd5a382acdcb4645e02bd84a5cd8c839243a9e3a743d453260ddcec2eed6e9dc2c7f70701778e454d99bd3a904b513ecd348427d9769030093c8fc7355a873456b72fc5081680b19036dc10e79e299cee4f9b42be1564f242aa23f0581fbc291a5aeae6caffab1f4a672b39c9e355b89a3b800a06fb9b739cf39c9a86edb9dd193e556122d7208d1628e32181e46073fad47b440f0d297bccd6d36e8ceeea7208e70462b48cae73558f29a42a8c82800a002800a002800a002800a002800a002800a0028011865483de80332eac9c04f22da2936b67e66da47d0d23784d7da660eaba8d8da48b69ae318cb8dc377381f854b378d4507cd0652b2f115a411fd91352b67891f01a46eabf8d09b5a1a54f6337cef7343fe121b4540c2f2c1ce7030e3fc69b93325084b4b9a075bd2961dd3ea96809ea16407f4aa4ee73b5ae871fe3c9f4ab8b081ec2f6395c4bcc692860011d71dbb50c1c9bd19cbf876fce9daddbdc803686dac4f4c1e0d48a3ab3bfbef12e976e1834c0cbbb3b94ee3f86286fb1d7c8a3ab7a149bc7cb167ecf68f337f7e46dbfa0aa472cdc5e88cfbbf1adc5ee04d630003a6d66047eb43410ab286c6a695e29b6d4a6b6b1b8b4689f7aec2a77027f98a194aa3d59df0e16839f739ad66e435d110aeede76920671ea6b19a6d9dd45251bb327ec10c13c4d079e1d98972e720d4ce2a27453aae49dcdef0fbcad7132b65900e1b1d3daaa99c75ed646f8ad8e616800a002800a002800a002800a002800a002800a002800a000d007977c5043fdb36c49e0c3fd4d4948e2368f4a63b86d14085ce3a7e74084ea6819d76a5a2cb178334bbd48cfcbb8cbc72031c834c5adce5f033c52486ddc9071d2aac028a6068682e63d76c9c76997f9d26847b581c52119f79a87d9a6310b595c819dc07152dd8d230bf5311cdceb575fe8ea5631ff2d390a3d707bd60ef366ea4a923a5b2b64b4b65863e8a393ea6ba22ac8e594b99dcb14c41400500140050014005001400500140050014005001400500140193ac78774cd69e37bf8599a31852ae54e3f0a560b99bff00080f87bfe7de6ffbfcd4c770ff008403c3dff3ef37fdfe6a02e1ff00080787bfe7de6ffbfcd405c07803c3c0e45bcdff007f9a80b9bd3e9f6d71a71b0923cdb94d9b41ed408c43e04d00ff00cb097fefeb503b87fc20ba00ff009612ff00dfd6a2e170ff00841740ff009e12ff00dfd6a770b9241e0bd12de749a28650f1b065cca7a8a5711d0e28010a83d68005455185000f41400b8a002800a002800a00ff00ffd9b91515ffd8ffe000104a46494600010100000100010000ffdb0084000d090a0b0a080d0b0a0b0e0e0d0f13201513121213271c1e17202e2931302e292d2c333a4a3e333646372c2d405741464c4e525352323e5a615a50604a51524f010e0e0e131113261515264f352d354f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4fffc000110800b400f003011100021101031101ffc401a20000010501010101010100000000000000000102030405060708090a0b100002010303020403050504040000017d01020300041105122131410613516107227114328191a1082342b1c11552d1f02433627282090a161718191a25262728292a3435363738393a434445464748494a535455565758595a636465666768696a737475767778797a838485868788898a92939495969798999aa2a3a4a5a6a7a8a9aab2b3b4b5b6b7b8b9bac2c3c4c5c6c7c8c9cad2d3d4d5d6d7d8d9dae1e2e3e4e5e6e7e8e9eaf1f2f3f4f5f6f7f8f9fa0100030101010101010101010000000000000102030405060708090a0b1100020102040403040705040400010277000102031104052131061241510761711322328108144291a1b1c109233352f0156272d10a162434e125f11718191a262728292a35363738393a434445464748494a535455565758595a636465666768696a737475767778797a82838485868788898a92939495969798999aa2a3a4a5a6a7a8a9aab2b3b4b5b6b7b8b9bac2c3c4c5c6c7c8c9cad2d3d4d5d6d7d8d9dae2e3e4e5e6e7e8e9eaf2f3f4f5f6f7f8f9faffda000c03010002110311003f00ee0367d3f2aa10e04e31400bd680002801c2801d4005002d0018a004c50014001340c6ee2a78a007e43ae2801f1b12b83da80149a4034b8a6051d5239678516dc12e1bb544d36b42e9b49ea436da0233092edc13fdd418152a9f72dd67b235e1b786dd36c31aa0f61d6ad2b6c64e4dee373f37e35449393814808c9c9a061400b400a2800a0033ed4007e140067da901982a892406818e1400a280173eb40c634f0a7de9147e352e49751f2b646d7f08fbbb9be82a5d5894a9c885b516fe08bf3350eaf645aa5dd913dedc3742abf414bda4994a9c51134b3b1e667fcea5ca5dcb518f6156e2753f2cae7ebcd25392ea3708be84f16a0e0e25407dc715a2abdcc9d15d0b497114838603d8f15a29c59938490eddb4e54e6ac9268e40791400e63c5022ac8e41a064b6ad9c8ef401705210d90e168404510cb6e3d29b02466cf7a431323d6801722801411400bc5002d001400500140196298878a0064d3ac29b9b93d87ad4ca4a28b8c5c9d8cf92fe673846083d8560eab7b1d11a315b9092ee72d2337d4d436dee5da285000ea69584c50c8298b503328f4a02c46d771af561f9d171f2b23377bbee066fa02697314a20b2cf9c885ff1e2a6ecad0991a43f7a3fd69dd8ac8933c72a4504d8afe748929c332d4a934cbe54d1a56d2c8d187ddd7a5744652b184a11bd8b4b73e5a33cc7e4519240e95a466fa99ce096c42f750491acb148ae8c32194e41ad0cac58b6917b1e680b17d0e452019273d4f0280216951b8e70281d84062a0351e0a763408702beb400f045002e680173400668016800a00cb14c424b2ac49b98fd07ad2949245462dbd0cc95da790b3fe03d2b9652e66754528a10055ea4548f514ed3de8191b5b86fe361f434582e3459a779643f8d160e617ecd6e3a82df56268b07332444853ee4683e828b05c93cca7610d6607bd1604c66f50796a0ab8f575ecd40325088e3e65068b264dda23915a34fdcbedc763c8a2ed6c34d37a985a9f882e6de392dcc78765203e7229c64d96e9c7a15343b878ecd22572e073cfbd29556d8d518451d15b5ccc390d494e44ca3134a1d4dd47ef101fa56aaabea60e8ae84aba9827e61c7a50aa83a3d89d2eeda4ee01f435a29c5993a7244b885ba11f9d55c9d44f2e3ecc3f3a0350daa3f8c7e745c7a886445eb22fe74b99072bec396453d1c7e747320e57d890134c91726980b93400bcd00644d3a42b96e4f614a52512a317233a495a67dcc7f0f4ae6949c99d0a2a2b418edb4549690e2d1ac48ecbc13c91d69e961eb71a64dc48815187d4e68f40db71570d097321c838e945b41396a37a26f7930b9e0e3ad161397610b1214a10cac700f4a7615c8a5bb863255e740c3a8a341a4d80b9492da392275e79273da9d9b5a13749ea654fadc2ae56277948ebb50d4bb9a24880ea97521fdd5b4c7fe0153a95a1325f5e4637496b281ea451a8f41c3c4b1c7c3fca7df8a2ec5cb1ee0fe228dd32a7345d872a398d635196eee91601b9b3c0aa846dab14a5d11b3a54771044a648323fd93d2b276bdd1a2bdaccdeb7b98d860361bd0f06a9321c59637fa9e280489d9e28da32e3008cf15764257639640c0940a47aa9e94584f4dc9c14f2d5b7f5a2da137d490600059f19e94c43b612400783de8b05d0d2b8e7783f85017275f9402060639a7615c05c73f293473584e37dc952edbbf354a6c874d13a5d21ea08ab53443a6c956446e8c2ad34c97168e50b176dcc493ea6b95bbeacea4ada0edc00a431a46feb40ef61de512aa03e31d298ae2846527e61f5000cd1701c8bb410adc1ea319a684c047c1f9f83d88e2980ad6e8f8dcfd3a638c5009d83ecd6d9dcc10b1ee54668102c56b1aaaa2200bd303a5171d85631673b149f5c0a4c690c7703a28a452437cccd171d8ab74904884491a37d40a4238bd7ed2de325adf119f45e2aa0ddc535a147479234bc43283c9c64d39a6d041a47a2d9a288c1c6462b348a6c9a4b4826eaa293435268aeda75c479f227caff75f9fd695997cc98ae2ed1977ab1c0c0200356468097aa72af27979e30cbb7347309c4b11cb85c07047d334ee4b4c7accca3024e3d08a2e1f22413be73e61cd171591209d8f561f95170b0f171cfdee45170e51c1b79ce46695c76b1280475a6891e314c5614669dc2c610c0a82c518ec2810edc7bd016144bb7b1a63b0c92e7038a4d8d109b873403b0a266eed4c4452dd9419cd0229c9ac4319c3b628023fedc87b3fe94ec035b5e41f7439fa29a02c56975e949f92190fb9a341d99564d72f319485b3f4345905d942e358d51fa80a3e94d288aecce7bb7924fdfa6f27fda355cbd857ee6edae98bb16495367190a2ad46c8894aef437f4dbb11916ee70bfc26b39c7aa2e2efb9a6d7210565734e5162bd04f34260d16d2e518738aab9361e44120c3283f514681aa2b3e996ec3311688faa1c51ca1cecac74fbd88f170932ff00b4b834ac5732011cca712211ee0e452b0683f6b81c669d84479914f22914ac491ceca69145c8e7661d3154990d58b11bfaf5a643260d4c4610c1e4d48ee3c328ef405c50c3d0d310b907b50035941fe1a561a6445541ed4142610f5229883ca89ba81400c3696c7ac6a7f0a62bb145b5aa8f9635fca80d463c7028cec5fca931a29cc6119fdd8fca916ae54778c03c014225b32af2447384193ed55662b8691a6799726ea70362741ea6b58a329336246c9aa24aecc334868962bb7ced91895f7ace515d0d232668c4032ee46fd6b3b1776484ca9d01fc28b0ae2adebaf5a43b9226a783839a2e16b9723d4558734ee2e52c2dc23d31598ef90f4a62d46f94adda906a27d980ed4ac3e60dbb7a530b8a1f1d4d21920940fe2a0463027b9a043c35301e1b1de8006942f534015a6bd45070fcd034674b7eccdc1a56655d025e31a761391335f18d32e303de9a4c572b3eb518e8ac7f0aae526e42dade7a46df9d2e51dc81b5499ce163033ea69f285cbd6b0493aee92ad4110e6c2eed001c0aae52548821d316439208039269a883958a971afd9dbbfd9e257289c640eb4ec45cb105c2ddc22588e54fe948a4c56422a59481533458772cc1bd3ee93438a62e668bd1ce71f3566e0fa16a6ba930749076351b1431ede27e831f4a43b109b46539473f8d1a0eec7abcf1ff00152b0ee4aba9327de19a3543b265983568c9c118fc697330702cb6a1177703f1aab93ca34df41fdf145c2c27da6271c352b82427991fad2b8ec67c8f1c6c55e740e3a8e6b4e532b92c5b65b6578f0589e4e69f2e82e6b32acd791c6c5448188eb8a561a65592fb70e0d3e561729c92ab1f9dc7d334f944e4314c6df74337d062a9445cc4a3cd03e450b4f910b988da091ce5b24fa9aab0ae33ec4e7b52b1571e9a7313d28e50e62ec1a4b123e5e69a892e67431dac50431875c6473835664d90cb6c2627cb8d48f50dd3f0a06992fd8d23b6003fd4e3ad026ce7eefc1d693ca6632345bce7039a00d0b5d016d214860c14f5f5fad2b0d309b4f5504ef52476c1a4d14a4325d3fe48da2000c65c934ac3b95d4286da1837be314032755c8c531128887f0f0694a098d4da17047de047b8ac65068d6334c7efc0ea0fd6a2c5dc8e69a3c74a1a63562a3857e8696a0ec11c100e59b9a7a889479038a2c05968ed6298090ed1b7239245534ae4ddb4488500dc026ceecad9c52686993c8b6e151bccfbc33d2938a1a9339bfb54c0e669914e382c8371ae9e5ee7373762bb5ccdb046b76aaa0ee002f23f1a760b91dc4b24e70f279847f105c668b0091da96ed4ec05c8ac01c645160b9a11592a8e94ec4dc9c5a8f4a2c171df655f4a2c171eb6809e94582e5a86c87a53b0ae5b100418502810ed87032dd3a714011c5ccc501038e805004e23c0c6723e94ec213610303047a114ac1702a720eee9d2801ae9b81ce39f6a06549ad99b1b64c15e98a5629333e7b5632648e7d40c66a6c55c1212bda988b0a94c43bcacd1615c865b62dd38a89534cd2351a2949a7cc4e438fc6a3d9b2bda219fd9f3019e0d3e461ed10c36d30fe1a5c8c7ce83ca99792b4b91873a2613c8f3604a0b631ca8e94f958b992331bc4566c8c8d71b40241017028f66c3da4443e24b31188dae772e38cae7147b361ed2255fb3e4e49c9ad8cc912dbda90cb50daf3d29d8468436e063e534ec2b97a28bd109a2c22ca4121e898a62b932d9b9ea68b05c996cc0a057255b703b50171f85518a421280108e28195adf22fdb3fdda16e37b1778a6486050026050034af14010ede690c56855c722819035b6d3d38a02e27938a005094001414006c1e94009b07a500218d3b8a0082758c21e28033a18732c9b002e51b60f538a48a7b1e6f710b239471b5c13b81ec6ada22e40d1fb52b01dddb44ce466958b6cd7b7b44e3229d89b9a315bc407dd140ae5948e31d10500598d148e0502260805002fca2801ad228a0085e7cf02801a1f34012020d002e334808d23c4ccc3ae29a1b1d834c42d001480539c500342e690c701400f001eb400d687bad0046d17a503186322810d294011b29a00af216140ca53335032bee20e7a1140c86f6d2cb51ff8fdb7cb8ff96b19daff008fad34c9713225f09dac8d9b6d47603da68ffa8a7742b3356d22923c6791486cd787a0ed4c92d20a00b31a13d680270428a402196802369a8021690b50301401228cd3112a8c5201c0d003a265f3f61eac38a064e61cd17111b4441a60204a400ca31400805218b8a003a5003d4d0038a06e475a008993d450030a5032364a00af247c50053962f6a00acf1707028022317ad0044d19073da819a51403d29905a4805302c471014012b32468a4faf6a4044d2072447b4fb739a06330a622c58f07d2801823caef2d85edc500384448054820f1400ed801c16e47b5004e91831a918cf7a0063b80700d00356424d3006c9915c751480d192758a3df21000192681198fe20d2cbf966ee20fe99aa5062e645b8ee51c0284107a114863d9c11d290c8f26900e0c68024c0cae4718ed40c36f1c01400e180a0e6801f8047cdde8023688e78e87bd0044ca39e7f4a006c9082011c0c7340152488668020687da80186dc7a50030db0f4a00b89181d6a8924deaa290114971d85004665240f9cf140c50ec4905cd003d3e5e8dc7d280245040386ebda8017193c93c5004abbbd68017cbc8033d2811208031f9b9a064a22451d050221678fcc088a09cf5a061a8d925fd84b6cce5378c6e1da84ec23951e0b2f36f9ae86ec0058679c7b569ed190a0ad63a7d3f4f8acad6382362ca830093c9a894aeee5a56562db4631c548c6141400d2b400a18f1cd0317713c6680141ed9a0050c4719a00706cf5a006ba9238a0085b20f53c50044dc9e6810d2a2801bb050046c00ed4015ccc7b1aa10c2e4d218004d003d52802555a603c2d0224543da90122467bd004cb1d0048a805201c4814015ae256c6128191db2303bc8a681968138e6810d6068010161de8014c9eb40c78218520176d0034afbd002608a0005002d001cd00383500054374a0089a3a008d908a006e3d6818bb01a0463aa935404c919a404cb150225588fa5004ab0fb5004ab0fad0048b181400f000a402f1400c67c50044cfef4010c8d807140cb16ebfba527a9a6264b8a003140085680229148a0646242a79a404e92a9eb4012e41a004c5001b6801314006280108140099c50038303d68002bf8d0031a2cf4a008ca15340155201e954225588520245451400f01476a007822800dc28010c8077a00634ca3bd20236b81eb40c8fcdcd001ba8019230db8a00bd6f9f213e94c4494009400b400d7a4321740476a008482a78a007a4a475a009d6406801e0e6800cd0021268018734009400ddc45003d65c75a00903ab5003b19f434014f001c16e7e94c448a32808eb9a0069600e33400c3328ef400c372beb40c8dae47ad0046d313eb48061627bd001400e00d003d738eb4c07b0036160707ae290cbf17fa95da06314c963ce36e734008071926801d8e9ef4011c8c3d69006dca823d39a0644e2802165f4a006e48a0648929ee681132be6801f40060d0037693400850d00298806e476a0630a30e78c7a8a0076e2801c9e680287da8ff007bf4a6481b8e31be818c79998fdecd003705bf88d0026c6a401b1bd09a000abfa500183e9400e1c75a00706a602ee07bd0038487230d9c74a00d18096857340873640f6a0081a7d9c0a008ccd2b1e28011bcf6ea7143182bc8a402dd290c943eeeb4085c0ef4011ba83d28022271da818a1c03401324a3bd022c2b06a0075002e2800ef9a062119a00694c8c76a00c5099ea29923844a7b5031c205ec6801c20f4340122c4477a4224031400bc1ea05002e17bad0034a467b50318628cd0030db83c2934c055b33fde14016e30f1c4141048a6238df12eb5af457860b38cc512ff00185c9354a3725bb0be1abed76f2f4a5e46cf6e179919369cff005a24ac34ee7671c2c1413d6a06236431cf3400d650dda90c8f6b29e2800cb1eb400a0668011909a0085a36a0633045003d2565a00b31dc03c134013ac80f434087e680141a007034018eb4c43c283da800c01400a2900f140050002800a063828c5003828f4a0429e2801858938a604b19c62801ce88fcb2827e94015a4668dbe438a602a4f21ef400f572c79a40482900ec0340c6328a004005003c28a0042a2802278d7d2802078d681919001e280268988ef4016918d02250680024e2803fffd9"
const privatePart = "0xf927b5f927a7b9132effd8ffe000104a46494600010100000100010000ffdb0084000d090a0b0a080d0b0a0b0e0e0d0f13201513121213271c1e17202e2931302e292d2c333a4a3e333646372c2d405741464c4e525352323e5a615a50604a51524f010e0e0e131113261515264f352d354f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4fffc000110800b400f003011100021101031101ffc401a20000010501010101010100000000000000000102030405060708090a0b100002010303020403050504040000017d01020300041105122131410613516107227114328191a1082342b1c11552d1f02433627282090a161718191a25262728292a3435363738393a434445464748494a535455565758595a636465666768696a737475767778797a838485868788898a92939495969798999aa2a3a4a5a6a7a8a9aab2b3b4b5b6b7b8b9bac2c3c4c5c6c7c8c9cad2d3d4d5d6d7d8d9dae1e2e3e4e5e6e7e8e9eaf1f2f3f4f5f6f7f8f9fa0100030101010101010101010000000000000102030405060708090a0b1100020102040403040705040400010277000102031104052131061241510761711322328108144291a1b1c109233352f0156272d10a162434e125f11718191a262728292a35363738393a434445464748494a535455565758595a636465666768696a737475767778797a82838485868788898a92939495969798999aa2a3a4a5a6a7a8a9aab2b3b4b5b6b7b8b9bac2c3c4c5c6c7c8c9cad2d3d4d5d6d7d8d9dae2e3e4e5e6e7e8e9eaf2f3f4f5f6f7f8f9faffda000c03010002110311003f00f4ea008aebfe3d65ff0074d0061d500500140052016800a6014001a4052bed4acec1775d4e91f7009e4fe145ec3b1c96afe34692378b4f89e33d3cd6ebf954f30ec7257124f3485e691dd8f2493487622da4727ad002ed2092cd814012413cd04aaf14a5181ce41c62803a5d27c6135b4a12f9ccd11efdc534d89d8ee2c6f60beb7135bc8aea7d0f23eb5422cd002d002d002e2800a003140050018a002800a00bfa5ffcb5fc3fad0c0bf484140115d7fc7ac9fee9a00c4a6014c02800a402d300a00290084500723e2fd127ba956f2dd4bed5c328eb52d151672125a4eca4790e31c676d67748d395b152c6ee424ac2df951cc86a0c99743bd60599428f7a5ed6257b16549ac1a36db238c8aa52b90e162a9895588273544588e4087ee839a04cd7f0b6a73d86ab100c7ca9182bafa8a607ab8aa10a2801d400b4005001400500140050025005fd2ffe5afe1fd6860cbf484140115d7fc7b49fee9a00c5aa00a002800a402d300a005c5002629011dc308e0763d854cdda2d9505792460ec04f415e71e9d894a285e83f2a77119b7c27642215cd545aea4cefd0e72ead64327ef81527d6ba2325d0e6945f52ab59f078fc6ab989e5209ac2455dc54e29a9225c58ed22d5e7d52de251cb38aa20f5f02ac4281400e1400b40050014005001400500262802fe99ff2d7f0feb43117a9005004575ff1ed27fba680316a802800a002900e14c05a002800a0064b0a4d198e550ca7a834981cddf5a3e7643338c1eaa71c5704aca4ec7a514dc55ca7e4ce1b62ace5bfbde6f14ee856b1a3142e8a3cd209acdb2915b53b54961c9238aa83b314d5d19d670471c2f2b2ee65e808ce7e94e6ddec2a715b972e62dfa6b34aa49284904743daa60fded0ba96e57725f0768c2084dfcebfbc93fd583d97d6bbe279b23aa02a8917140c28016800a002800a0028012801714017b4eff969f87f5a188bb48028022b9ff8f693fdd34018b5402d0025002d201d4c05a002800a00461952338c8eb49ea34ecce5afe69adef9e1653b3b311d7dfad714e9d99e853abccaf61f0cd1bf01c16c77ac5a68d2e9849388ce19b9ec284ae26ec549a6f3c819f96b48c6c67295c96ce3d983e9da94d15065c9d7cc8761ef511d184b536ed1556d2254fba14015e945dd5cf3e4acf526c53243140c2800a005a002800a004a005c500281401774ff00f969f87f5a188b948028022b9ff8f693fdd34018d54014005002d2014530168016800a0028031bc450af9093ec2594e09c6702b2ab1bab9bd1a8e2ec73734f6ebfeaf6123bf4ae65193dce994d15e5bec8fbdcfd2a942c439dc75bc935c105130077343b212bb356d7ccced23ea6b2933489a6838e6b22c545b87beb68eda560a5c191074dbdcd74e1e52bd8c2b4636b9b9226c6c76ed5d8718da00281850014005001400500281400e0281172c3fe5a7e1fd68605ba40140115d7fc7b49fee9a00c5aa016800a005a0075002d0021603a9a0042fc640a2c021f3197e5e29d808994f46e7d6981045676ff0069c882207d760cd2b0db31b5ed2e18ef3ed60651b0b281fc3e86b1af1d2e8da84ba32286d56350636054d71395cebb1663017a5485874b31002a0dce780a3bd38c6ec1bb1d1e97642d2dc17e667e5dbfa577c20a2ac8e0a937364f21df2ed1d1473f5ad11034c6bf4a00698cf639a2c0308a4312800a00280171400e14085a00b761ff002d3f0feb4302dd200a008aebfe3da4ff0074d0062d500b400b4005003a80109cf029a404a91830f4ef400a6350327a0ed400a8a7a9a006ba027a50037ece37120e280201a745860d972c771ddce6985caf77a5a436c66b41803964edf857255a29ea8e9a559ded228dadbbdd90b0ae33d5bb0ae7853727a1bcea28ad4d01a5c56e17cb72661fc43ad77d3a5189c73aae45fb56b958b64f8623eeb67afd6ada57d0cc95136ae3a9ea4d002e3340098a00691c60f3400df2988240a560184107918a430a005a005140809a00b7a79ff59f87f5a181729005004575ff001ed27fba680316a802801680168014f4a00147354059523cb23f1a4031086219ff00e0228025e0f4a4026298050004500452b3b46f1c685f3c301ef4340245669146b1afca8a3ee83d7eb4a368ab21b6e4eec995550614014c42d003a90062800c50003ae05003fe94804640c306802ab295620f6a062502026801a4d005cd37fe5a7e1fd68605ea40140115d7fc7acbfee9a00c4aa01680145002d0004f229a014039f7f434c08e79645185ce69a016190b20661cfa5202da3e45201f9a002801aedb54b7a50045a717712c8c382dc50c0b2dd6900da601400a2900bd6801aec53e63d3bd301038c1607af4a2c039181a403c1a0082e3ef8fa52022cd031a4d002134017b4cff0096bf87f5a188bd48028022baff008f597fdd3401875401400ea0051400a17753403d72383c8f4a604370d8ff00ebd34056b798190c7dfad202e23e3bd302756f7a403b781de95808e597e42a0673ed4d20248124500ee5f2f6f0bb79cfd6a5ee21c73ebfa531899c7bd0026ea004dd4c0707a4050d5aefc931c287e79339fa53403ad43bc40b1e3b6698165038fbbf99a902740475a404773f787d29015c9a0621340099a00bfa5ffcb5fc3fad0c45fa40140115dffc7acbfee9a00c2cd500b400a2801680268c62a80936f7a0452bd04034c667592cbf6f919d408cc60213d49cf3486cd121c7404d02143b0ea0d004a8c87ef161f5145c06b83e6a957073d39a622d1728b82c0e2958088cdeff0090a2c31a66fafe54ec030cf40009b3401207c2e7bfd33480c0d51f6ea96eeef80415c11dcd36346e5a9c20a188b919cd4b024029010dd2fc81bd0d202a6681899a00334c0bfa57fcb5fc3fad2623429005004377ff001e92ff00ba68030aa8051400a2801e8326840588c103daa98879e05202b5c80ca6a90112a63cbc0fbbd6a56e32622a8055a40480e68018986ba551ce393474112ac11400ac60804963924f27eb4900a554d032368853b80c310ea31f8d3b806c41cf287dba52021908cf249fc29a033a6812faf51482523e4d2931a35221b0e053e822ca1c1a9604e0d201b30cc4c3da9019f40c4a60250068693ff2d7f0feb4988d1a4014010ddffc7a4bfee9a00c1aa01680145004f02e4e3d69a02c9e0741f89c502216763d1401ec6a920216c96e686ec8051938dbd6a10c7b62a804045002861d0e7f0a0058592395a4621502e492718a1ec22595c3302a72319069201a0f1c5318a1b34008ccbf8d004323714d01160904d301b6b0889598fde739359bdc09b3839aa404c8d9a009d1b3520484654fd2901967834c62502133401a3a47fcb6fc3fad26068d200a0086f3fe3d25ff0074d00605500a28016802dc1f76a8448cd81c019f5348081e4939f98fe000aa490112fa9c927d6a58c962e5a84023039e6a80147a5003ce14649a402da2a4c25f31032b70430c834a422490056c2800018005080603834c60cbfc4b4006e0c30e307d680227551f79853b80dfbdc0e05310b598c6b0c8a69d8011f69e1d00f7354c0b51963d1e3fc05481615b8e4827da9019b2f1230f7a006500250068e91ff2dbfe03fd693034a9005004379ff1e92ffba68039f15402d00385005984d5887b1a40447a9aa023cd663268b85cd3404b80eb9efde98109519eb8a0014e063191400eb6611b1e4e19b038a1889188627eb400c34c601b145806b39c71d68b011e09249e6980e4ebcd0c436b31886801cabc6738fc2ad3d00b1182170791ef5204ea78a40674f8133e3d68023a06250234b47ff96dff0001feb4981a548028021bcff8f497fdd3401cfd500b400a3ad00598c1eb5621e68022638269f4022cd663275c8500fa55a10bbca1dc3a516b80e2d1bf24107da959811c8fb54ed14c658b51b6dc7bf352f7111337cc4d55800f3d2801bb58f6a063bca38cb9dab45c446cea7841c7f3a69006428cb753400d26b31899a009547207a5574025068026538152066bb6e763ea6801b4009401a7a3ff00cb6ff80ff5a4c0d2a4014010de7fc79cbfee9a00e7aa805a00721018123229a11612488fdd2d8f715404a48c707f3e2901048c31818fc29bd80afbb74cb18fab7d2a5202f70c9838cf63542223ce51a9811e4a9c1a602312d4016166c26cc36426738e3f3a8ea31aae0d55843c62900a655419a5602bb3bccdcf4abb580911420dc7a0a4ddc08ddb79cd27a21a13350301f785004f1f26ad812aa9271834842c80ac4c4f1c52199f4862500250069e8dff002dbfe03fd69311a748028021bcff008f39bfdc3401ced500b400b408b762a183e4671d29dc0b9b548e5452b8142f36a1e062aafa0152cdc48eef8c027154968265b1279670c32b46e049b51fe656a5701ae8bd7bd170216fb869816e5f92dc8ed8152b7194b38e41ad043c494ac01b59cf38c517b012655075a5b811c8e5f03b534ac0267a8a52d8684cd663150fcc29a02da29dbd80f5ff00f5d3002a4f4918fd1a9015a625494c93eb43022a4312800a00d3d1bfe5b7fc07fad26234e900500437bff1e737fb868039daa016810b40152fae2fad61696d265551f794a039fc6a66da5745d349bb32a45adea4cbccaa7fe022b95d691d8a840a9ab6a7a8b69b71b7f79214c0c0c11ef5a42bb7a332a94125789ca68de2dd474c510b85b8894fdc93a8fc6ba1499cd63acb2f1d69372025e24b6ac7b91bd7f31cfe94f9856366d6fecee79b2bc82607b2b8cfe5d6a934c0b46670306334c43379240f5205302e5ce4447d33511dc194c8cf4ad00434000de7a66802408f8e84d2ba02bdcde5ada296b9ba86203fbee07e94732198cde2dd31af22b5b6324ed2385f300daa3f3e4d439dc2c6f835232c59a6e7c9e82988bf48086e828859b0323a1a0666e68185020a06252034f45ff96dff0001feb4311a948028020bdff8f39bfdc3401ce8aa0145002d022a6a8c56cd80eac7159d57689a52579232e1418c57033d144926110b1ed4206721e29b058992fa150164e1f1ebd8d75519df46725785bde460706ba0e61bca9ca120d202e5b6b7aa5a60417f73181d84871f953b81a70f8d35c8f1bae63971d3cc894feb8a2ec0b0de3ed71b1f35be3d3ca1cd1700ff0084ef58feedae7d7caffebd3e6616236f1b6bac789e14ff007605fea295d811bf8c35f718fb76dff76241fd28b819f73ac6a775ff001f17f72e3d0c871f9500512c58e4924fbd0059d3033ea56ca9f78c8b8fce901eb924eb0a80dcb1ed4a751416a690a6e6f43326965690b07653fec9c5714aa49bb9db1a514ac46d77a828022bb994f40739fe7571a92ee4ce942d7b1b16cf7c20d9797427ce0e7cb0a47e55d8afd4e176e84b4c0290094009401a9a2ffcb6ff0080ff005a188d4a40140105effc794dfee1a00e705500ea0428a0082fa169adcaa004839c7ad67522e51b234a725195d9886568db063208e08ae2713d0525d08358f3e7d2a5fb2e44a06401d69c2ca5a933bb5a142c8aeb1e1d36d2f3328dac3b823a1ab69c67744a5cf0b338d6568a468dc619090735d699c2d586927da810d2deab400c2476cd002839ef400e007f7a801428f5cd003b18a00298084d2036bc20aa75f81dc642066fc7149b495d8e2b99d91db5cdce2e543e4b49c938e057136e7791e845282b22c000ad66683245c818aa8ee4cb635d7ee2e7ae2bd1479a140050014009401a9a27fcb6ff0080ff005a188d4a40140105effc794dfee1a00e705500a280168014500666beab169f25caa8f313183eb59d4826ae694e6d3b1ca7f6b5ca5baba84c91cf06b9f915ce9537633ef3529e184cb0ac7148dd5d1706ae304dea4cea34ae8c0791e591a4918b3b1c927bd6e958e56efab0a0434d0031ba50008320d004800a005005002f4a0033400d279a00eb3c190a1f3e5232e0800fa0ae7afd8eac325ab379efa787518ad50af973a6e7c8e7f3a57e58591a72a94eefa0eb899d3ee9c5608dde887da48f24f1c6cdf2935a4229b329c9a89bf5da700500140094009408d5d13fe5bffc07fad0c0d4a407ffd9b91473ffd8ffe000104a46494600010100000100010000ffdb0084000d090a0b0a080d0b0a0b0e0e0d0f13201513121213271c1e17202e2931302e292d2c333a4a3e333646372c2d405741464c4e525352323e5a615a50604a51524f010e0e0e131113261515264f352d354f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4f4fffc000110800b400f003011100021101031101ffc401a20000010501010101010100000000000000000102030405060708090a0b100002010303020403050504040000017d01020300041105122131410613516107227114328191a1082342b1c11552d1f02433627282090a161718191a25262728292a3435363738393a434445464748494a535455565758595a636465666768696a737475767778797a838485868788898a92939495969798999aa2a3a4a5a6a7a8a9aab2b3b4b5b6b7b8b9bac2c3c4c5c6c7c8c9cad2d3d4d5d6d7d8d9dae1e2e3e4e5e6e7e8e9eaf1f2f3f4f5f6f7f8f9fa0100030101010101010101010000000000000102030405060708090a0b1100020102040403040705040400010277000102031104052131061241510761711322328108144291a1b1c109233352f0156272d10a162434e125f11718191a262728292a35363738393a434445464748494a535455565758595a636465666768696a737475767778797a82838485868788898a92939495969798999aa2a3a4a5a6a7a8a9aab2b3b4b5b6b7b8b9bac2c3c4c5c6c7c8c9cad2d3d4d5d6d7d8d9dae2e3e4e5e6e7e8e9eaf2f3f4f5f6f7f8f9faffda000c03010002110311003f00f4ea002800a002800a002800a002800a002800a002800a002800a002800a002800a002800a002800a002800a002800a002800a002800a002800a002800a002800a002800a002800a002800a002800a002800a002800a008ee278ada169667088bd49a00c2bed7951d524ba8acc37dd8f879dff00e03d13f1cfe1401aba7dd4b751ef7b768a3c0dacf22b337e0b903f3a00b9400500140050014005001400500266800a00514005001400500140086800a00334008ce114b390aa0649270050024722ca8b22306561904771400f140050014005001400868022b99bc888bec676e8a8bd58fa50053b9d262bf657d45e49429ca46b2144438ff6704fe39fc280256b4d3edaddb75b5ba44a3e6250631ef40146ef4c4b585a7d20c3a7b9656771f2a63dd31839fc0fbd005bd335317924d6f2a79575011bd3390ca7eeba9eea7ffad401a1400500140050021a004a005cd00267da800cfb50028a005a002800a0028010d00453cf1dbc5e64a76ae71ee4f603d4d005497fb4ae31e4b456884f3b97cc7c7e7807f3a00867d1e49c82daadf82a72394db9f75db834019d730eafa76f9649c5d418f9e745db301efd405ebca8cfb5006ee9d24b35aacb24b048ae0143092c31fef1ebf5e2802dd0014005001400868033d2569f5a953fe59dac4bc7abb67f901ff8f5005fcfb50035d15f1b973b4e47d68039cb4d1b539df58b7d66f5a6b3ba901b6d8d868803918f4c7cbf88a00b3a8db0d37fb32f626626da45b7919babc4e429cffc0b69fc28037a800a002800a0043d280133ed40067da801d4005001400500140050014005006747fe99abcaedcc56444683b1908049fc01007d4d0068d0014015ae6fadad248d2e6411894e159b8527d33d01a00cdb241a56b8d631f16778ad340bda37046f51ec721b1f5a00dba002800a002800a00ccb06035dd5233c31f29c7ba95c7f353401a74005004173796b689bae678e31db73609fa0ef40189adbdcea3691a22bc103dc44a81861e63bc1e9fc2a0027d4e3b63900e8a800a002800a002800a002800a002800a002800a002800a0028033741e6d276ce58ddcfbbf09580fd00a00d2a0063cd124891bc88aef9d8a4e0b63ae3d680327c55a23f883473a7adcfd9c348accdb73900f2280219f4f834fbcd161b30eaa2edb86919b03c89338c9e074e071401bf4005001400500140195ab5b5cc77316a9a7a7993c2a5258738f3a33c903fda0791f88ef4016f4fd46d75180cb6b26eda70e846190fa30ea0d004b736d15d43e54db8a139f95ca9fcc106802b4563a6e9cad3a5bc10ed1969481903dd8f34011db06d42f12f5d596de2045bab0c1627ab91f4e07b13eb401a54005001400500140050014005001400500140050014005001401890ceba4eb72dadc304b7d424f36ddc9e04981b93ea71b87ae4d006dd0073fae58ddcf2ceeb6c2e6395523528e03c2a1b2c541efdf20e781e940142d3c4af6171f67bef365872c46f4227854138dca7961852723903ae7ad006a2dec17576baa6e2f691298edb6a926676fbc54753c0c0ff8176a00b6757b5744fb2efba9245cac70ae4fe39e17fe04474a00ab347aedd4a41fb3db427a0498eefc4edfe447d68027834cb88e32b26a133b6c281ce723272c79279e807a63bd0069018007a5002d00140142f747b3bc985c32bc5720604f0b947c7a64751ec7228033efadb52b3583cad6ee4ac93a4477c5131018e3aeda00bd16910f98b2ddcd35e48a72a676caa9f50a30a0fbe33401a34005001400500140050014005001400500140066800a002800a008ae678ed6dde798e11064ff009f5a00e734fbed2f58491f551b66b82d1886e576045071b549e339193839cfd05005f4b5d5b4f18b1b88efadc7dd8ae98aba8f4120073f88fc68025fb7ea9d0e8a41f53729b7f3ebfa500665dac9ad308aea086e151b3e4dbfcca0ff00b53103f2519a00bd3e9d2bdb092eeed6d85baef845b28558081d72796e323b0209e280301a7748e5b9dccd2a98e7ba585da241ce30a3f8a46c72338edd79a00ed609639e049a160d1c8a19587420f4a007d0014005001400500737e2792e6ec25bd8cde4982e60cc98c8f30baed5fa01c9fa8a00d9d32f56fec926da637e5648cf58dc70ca7e86802dd0019a002800a00322800a002800a002800a00434009400b4006680128033f9bfd4c83cdb59b74fefcbfe0a3f53ed4014f518e2d36f9eea68d1f4dbc216e95d72b1bf41211e87807f03eb4016ffb0ec979b56b8b5f4104eeaa3fe039dbfa5003d34a43c5ddcdcddaf649986dfc42800fe39a00bc02451e0054451f40050067297d55c3f2b603a7adc7bffb9fcfe9d4016eb4c42b13d9e209adf1e585e1081fc2474c1cf5ea28020d36f3cabcfb1187ca46190a0f113f529f4e720f7e7d280363340050028a002802aea1726da01e52879e46d9121fe263fd0724fb034019f7b6a2d6cac600c5d8de44cee7abb6edc4fe273400b799d2752fed05e2cee484ba1d91ba2c9fc81fc0f6a00d83c8a002800a00cabe927baba36b6b72f16c1f379406493eac47007e66802fc302c2f2ba924cac19b3ea140fe940138a002800a002800a0043d280139f4a0039f4a0039f4a00ad7f726ded59940f358848c1eee781fe7d28025b383ecd6b1c5bcb951cb37563dc9fad004924692c6d1c8a1d1810cac3208f4a00c38647f0fccb6b74ccda5b9db04ec73e41ec8e7fbbe8df81ed401b53cf15bc2d34d22a46a32589a00a02097546125da18ecc1ca5bb7593ddfdbfd9fcfd000691e9c500273e94018de23d3e6b9b6fb4da3c897100dc047d5c0e781fde0791f97426802d687a9a6ada5c374850b1189029e15c7514017f27d280145000cc154b310001924f6a00cfb006f6e0ea3202108296ca7b2776fab607e007bd0026abf35de991fadd67f28dcd005f9a28e785e19503c72295653d083d450065e933496933e9174c5a485775bbb75962edf8af43f81ef401a72ca90c4d2caca88a32ccc7000a00a25ae35151e4b3dbda9ff9698c3c83dbfba3dfafd3ad005e8d1624091ae1474a00764fa50028a005a002800a002800a002800a002803364ff49d7a28faa59c5e69ff007df2abf9287fce8034a800a0064d1473c4d14c8af1b8c32b0c8228028d9e8f05ab2fef6696388e608e56dcb17d3d7db39c76a00d1a002800a0028039cb0b46d1bc5771144b8b1d514cca0744997ef0fc473f850074740157539a5834e9a5807ef157238ce3de8032e79a69e71677c5a3b79ae9d189180ca305101f461ce7be08ef401bc000303814019b79f3eb9a727f716597f2017ff66a00d2a00c7f122016915d26e49ede50d1ccaa5847ebb80e4a9e87eb9ed401259c275158ef2f258664fbd1c50b6e894fae7f88fb9031e9de80352800a002800a002800a002800a002800a002800a00cdd27f7975a95c1fe3b9d83e88a17f9e68034a80192cb1c3134b33ac71a8cb331c003dcd0043657d6d7f1b3dac9bd51b69c82083d7a1f620fe340166800a002800a002802a6a504b3db62dd99655391b64d99ec46707b1f4a00b11071128931bc019c74cd003e8021bbb686f2d9edee13746e3047f507b1f7a00cc8efa7d29c5beacdbadc9c457b8e0fa093fba7dfa1f6a009e33e6f88e561cac36aa01f77624fe8828034a800a002800a002800a002800a002800a002800a002800a002803334139b0964fefdd4edff915a801d26a66691a1d322fb4c80e1a4ce228cfbb773ec327d71400b16961e459f5193ed7329caee188e33fecaf41f5393ef40105effc4bb5886f8716f758b7b8f456ff00966df99dbf88f4a00d7a002800a002800a002800a002800a00474591191d432b0c1046411401cce9f0cd05e6a0344921d904de5b59c84e30141e1baaf25b0391ec280352df5bb57945bdd86b2baff9e3718527fdd6e8df813401a4ac1802a4107b8a005a002800a002800a002800a002800a002800a002800a00e7747b07b98ae2daf6e19a1b6b9923f2106d561bb702c7ab6430e381ec6803a08e348a3548d15114602a8c002801d40105edac57b672dace331caa54fafd47bd0054d12ea59addedaece6eed1bca94ff007bd1fe8c307f3f4a00d2a002800a002800a002800a00334005007356e1ad6dbfb62204ec9e717007f1c5e6bf3f55ea3db3eb401b3a85859eaf6460ba8d6585f0ca7d0f620fad0065e9da2dde9174f2c57c1ed3cb6cc2b0e0b1ec700e33f4033e9401a9a74978f128bc8c2b2c6818ff0079f196fc33c7e740172800a002800a0033400500266800a0051400500140050065e9ff00bbd735587fbc62987e2bb7ff0064a00d3340050014019d7508b7d522d4518a874f227007de19f90fe049ff00be8d0068d00579eefcaba86dca90670c237c6577019c1efd013f81a006695752dd5a16b84559a391e2902fdd2549191ec7ad005ca002800a0043400940013804d0051d1429d1ad88428244dfb49ce3773fd6801ba6a9b3964d3b6910c6035bb139ca1fe1ff00809e3e98a00d2140050014005001400868012801680139f4a00327d280145002d001400500659fddf8a97fe9bd91ff00c71c7ff17401a6680139f4a0039f4a008aee0fb4da4b6e49512295dca70467b8a00c4361a8ea90083520d144aa11f0e3e7ec5b03ae7dfa7a13cd006e9b78dcc2cca0985b727fb27057f91340122aaae7680327271eb400b40050014008680139f4a0082fa530585c4dd3cb899bf219a006e98ae9a5da23e599614049ea4ed1400cd4ade69a0592d895b881c491f3f7b1d54fb1191f8e7b5004f657315e5aa5c42728e3b8c107b83ee0f14013d00140050014008680139f4a0039f4a00750014005001400500140197a8feef5ad266fefbc909fc50b7f341401a314d1ca5c46e1b636d6c763e9400fa002800a002800a002800a002800a002800a00cef1092341bd03abc4507d5b8feb401a08a15028e80605002d0065463fb3f5b310e2deff002ea3b2ca3aff00df439faa9f5a00d5a002800a002800a002800a002800a002800a002800a00cdbd517b7f6d6f1f5b6916791c7f0f0405fa9c9fc33ed4013d8c0f14b772c80033cdbc01d805551ff00a0e7f1a00b7400500140050014005001400500140050014019fab0138b7b21f7a69558fb2a30627f403fe04280342800a00ced7a1924d35a580667b6613c43d5979c7e2323f1a00b96b3c7756b15c42dba39503a9f504645004b4005001400500140050014005001400500140105edcada5ab4a54b9180883abb1e001f53400dd3ed9adadf123079a425e571fc4c7afe1d87b0140166800a002800a002800a002800a002800a0028002703268033b4cff4a9a6d45beecbf241ed18efff0002393f4db401a3400500417b72b69672dcbab3244a5982f5c0eb4019da11fb2cb77a593c5bbf990fbc4f923f23b97f01401b1400500140050014005001400500140066800a00cd8ffd3f523375b7b42563f4793a31fc3a7d777a5006950014005001400500140050019a002800a0033400500676aaed3797a744c43dd64391fc110fbc7f1fba3ddbda8034111634544015546001d85002d0014011dc44b3dbc90bf2b22953f4231401813b3e990697aa5c861e4462daec8524ed23ef71e8c07e66802ee9fabcba8dced86c644b62a4f9ee7183db8c60fe04d00166fa80bf10ccecf146d2a990a01bd708549c77f99871d7068035a800a002800a002800a004340094019d75792432bac8fe5ac8e22846dce7e5cb31fa73f95005cb54863b68d2db6f94061769c8a009a80145001400500140050021a004a005a002800a006c9224513cb236d4405989ec0500646837b6fa84d7178241f68908fdd30daf1443ee820f3cf5fc7da8036a800a0028010d0025001400b4009400e1400500140050014011ce262a3c868c1ff6c13fd68020db7f9ff596f8ff00ae6dfe34015efac6e6fa0f2667b7c0219595583230e8410dc1a00ada6e95a8d84f34bf6c8656980de0c440661fc58071b8f73df1401a206a19e5edbfefdb7f8d00336ea99e25b3c7fd726ffe2a800dbaaffcf5b3ff00bf4dff00c55001b755ff009eb67ff7e9bff8aa0036eabff3d6cffefd37ff00154006dd57fe7ad9ff00dfa6ff00e2a800dbaa67996cf1ed1b7ff15400edba8e7fd65ae3feb9b7f8d0005751cf125aff00dfb6ff001a0002ea58e64b5ffbf6dfe3400a06a19e5edbfefdb7f8d0030a6a8723cdb2c1ec626ffe2a8030e5f0d6a41d1ad350b780447308f298f927d14eec853fdd391401b41759c732d8ff00dfb7ff00e2a800dbac7fcf5b1ffbf6ff00fc55001b758ff9eb63ff007edfff008aa0050babff0014965f846ffe3400edba9e7efda63fdc6ff1a000aea79e1ed3f146ff001a0000d531cbda67fdc6ff001a00551a8e7e67b5fc11bfc6802cc41c463cd2a5f1ced181400fa002800a002800a002800a002800a002800a002800a002800a002800a002800a002800a002800a002800a002800a002800a002800a002803ffd9cac401020380c402800103"

func generateFlip(minSize, maxSize int) (privateHex, publicHex string, err error) {
	// TODO
	return privatePart, publicPart, nil
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
	// TODO
	return
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
