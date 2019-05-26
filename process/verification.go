package process

import (
	"encoding/hex"
	"errors"
	"fmt"
	"idena-test-go/client"
	"idena-test-go/common"
	"idena-test-go/log"
	"idena-test-go/scenario"
	"idena-test-go/user"
	"math/rand"
	"sync"
	"time"
)

func (process *Process) test() {
	log.Info(fmt.Sprintf("************** Start waiting for verification sessions (test #%v) **************", process.testCounter))
	wg := sync.WaitGroup{}
	wg.Add(len(process.users))
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
	ok := common.WaitWithTimeout(&wg, process.getTestTimeout())
	if !ok {
		process.handleError(errors.New("verification sessions timeout"), "")
	}
	process.assert(process.testCounter, es)
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

func (process *Process) testUser(u *user.User, godAddress string, state *userEpochState) {
	process.initTest(u)

	if !u.Active {
		log.Info(fmt.Sprintf("%v skipped verification session due to stopped node", u.GetInfo()))
		return
	}

	process.passVerification(u, godAddress)

	process.collectUserEpochState(u, state)

	waitForSessionFinish(u)

	log.Info(fmt.Sprintf("%v passed verification session", u.GetInfo()))
}

func (process *Process) collectUserEpochState(u *user.User, state *userEpochState) {
	identity := process.getIdentity(u)
	state.madeFlips = identity.MadeFlips
	state.requiredFlips = identity.RequiredFlips
}

func (process *Process) passVerification(u *user.User, godAddress string) {
	process.submitFlips(u, godAddress)

	waitForShortSession(u)

	log.Debug(fmt.Sprintf("%v required flips: %d", u.GetInfo(), process.getRequiredFlipsCount(u)))

	process.getShortFlipHashes(u)

	process.getShortFlips(u)

	process.submitShortAnswers(u)

	waitForLongSession(u)

	process.getLongFlipHashes(u)

	process.getLongFlips(u)

	process.submitLongAnswers(u)
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
	submittedFlipsCount := 0
	for i := 0; i < flipsToSubmit; i++ {
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
	ceremony := process.sc.Ceremonies[process.testCounter]
	if ceremony == nil {
		return nil
	}
	return ceremony.UserCeremonies[u.Index]
}

func (process *Process) getRequiredFlipsCount(u *user.User) int {
	identity := process.getIdentity(u)
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
	epoch, _ := u.Client.GetEpoch()
	log.Info(fmt.Sprintf("%v next validation time: %v", u.GetInfo(), epoch.NextValidation))
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
		u.TestContext.ShortFlipHashes = shortFlipHashes
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
		for _, h := range u.TestContext.ShortFlipHashes {
			if flipsByHash[h.Hash] != nil {
				continue
			}
			flipResponse, err := u.Client.GetFlip(h.Hash)
			if err != nil {
				continue
			}
			flipsByHash[h.Hash] = &flipResponse
		}
		if len(flipsByHash) == len(u.TestContext.ShortFlipHashes) {
			var flips []client.FlipResponse
			for _, f := range flipsByHash {
				flips = append(flips, *f)
			}
			u.TestContext.ShortFlips = flips
			log.Info(fmt.Sprintf("%v got %v short flips", u.GetInfo(), len(flips)))
			return
		}
		time.Sleep(requestRetryDelay)
	}
	process.handleError(errors.New(fmt.Sprintf("%v short flips waiting timeout", u.GetInfo())), "")
}

func (process *Process) submitShortAnswers(u *user.User) {
	answers := process.getShortAnswers(u)
	_, err := u.Client.SubmitShortAnswers(answers)
	process.handleError(err, fmt.Sprintf("%v unable to submit short answers", u.GetInfo()))
	log.Info(fmt.Sprintf("%v submitted %d short answers: %v", u.GetInfo(), len(answers), answers))
}

func (process *Process) getShortAnswers(u *user.User) []byte {
	userCeremony := process.getScUserCeremony(u)
	var answers []byte
	if userCeremony != nil {
		answers = userCeremony.ShortAnswers.Get(len(u.TestContext.ShortFlips))
	} else {
		for range u.TestContext.ShortFlips {
			answers = append(answers, 1)
		}
	}
	return answers
}

func (process *Process) getLongAnswers(u *user.User) []byte {
	userCeremony := process.getScUserCeremony(u)
	var answers []byte
	if userCeremony != nil {
		answers = userCeremony.LongAnswers.Get(len(u.TestContext.LongFlips))
	} else {
		for range u.TestContext.LongFlips {
			answers = append(answers, 1)
		}
	}
	return answers
}

func waitForLongSession(u *user.User) {
	waitForPeriod(u, periodLongSession)
}

func (process *Process) getLongFlipHashes(u *user.User) {
	var err error
	u.TestContext.LongFlipHashes, err = u.Client.GetLongFlipHashes()
	process.handleError(err, fmt.Sprintf("%v unable to load long flip hashes", u.GetInfo()))
	log.Info(fmt.Sprintf("%v got %d long flip hashes", u.GetInfo(), len(u.TestContext.LongFlipHashes)))
}

func (process *Process) getLongFlips(u *user.User) {
	u.TestContext.LongFlips = process.getFlips(u, u.TestContext.LongFlipHashes)
	log.Info(fmt.Sprintf("%v got %d long flips", u.GetInfo(), len(u.TestContext.LongFlips)))
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
	answers := process.getLongAnswers(u)
	_, err := u.Client.SubmitLongAnswers(answers)
	process.handleError(err, fmt.Sprintf("%v unable to submit long answers", u.GetInfo()))
	log.Info(fmt.Sprintf("%v submitted %d long answers: %v", u.GetInfo(), len(answers), answers))
}

func waitForSessionFinish(u *user.User) {
	waitForPeriod(u, periodNone)
}

func (process *Process) getCurrentTestEpoch() int {
	return process.testCounter
}
