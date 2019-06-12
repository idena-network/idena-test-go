package scenario

import (
	"idena-test-go/common"
	"time"
)

type Scenario struct {
	EpochNewUsers        map[int]int   // Epoch -> new users count
	EpochNodeStarts      map[int][]int // Epoch -> nodes to start
	EpochNodeStops       map[int][]int // Epoch -> nodes to stop
	EpochNodeOnlines     map[int][]int // Epoch -> nodes to become online
	EpochNodeOfflines    map[int][]int // Epoch -> nodes to become offline
	EpochTxs             map[int]*Txs  // Epoch -> Txs
	EpochDelayedFlipKeys map[int][]int // Epoch -> nodes to provide delayed flip key
	CeremonyMinOffset    int
	DefaultAnswer        byte
	Ceremonies           map[int]*Ceremony // Epoch -> Ceremony
}

type Txs struct {
	Period time.Duration
	Users  []int
}

type Ceremony struct {
	UserCeremonies map[int]*UserCeremony // User index -> UserCeremony
	Assertion      *Assertion
}

type UserCeremony struct {
	SubmitFlips  int
	ShortAnswers AnswersHolder
	LongAnswers  AnswersHolder
}

type AnswersHolder interface {
	Get(count int) []byte
}

type Assertion struct {
	States []StateAssertion
	Nodes  map[int]*NodeAssertion // user index -> node assertion
}

type StateAssertion struct {
	State string
	Count int
}

type NodeAssertion struct {
	MadeFlips        int
	RequiredFlips    int
	AvailableInvites int
	State            string
	Online           bool
}

type AnswerRates struct {
	Left          float32
	Right         float32
	Inappropriate float32
}

func (answerRates AnswerRates) Get(count int) []byte {
	leftCount := int(float32(count)*answerRates.Left + 0.5)
	rightCount := int(float32(count)*answerRates.Right + 0.5)
	inappropriateCount := int(float32(count)*answerRates.Inappropriate + 0.5)
	noneCount := count - leftCount - rightCount - inappropriateCount
	var result []byte
	for i := 0; i < leftCount; i++ {
		result = append(result, common.Left)
	}
	for i := 0; i < rightCount; i++ {
		result = append(result, common.Right)
	}
	for i := 0; i < inappropriateCount; i++ {
		result = append(result, common.Inappropriate)
	}
	for i := 0; i < noneCount; i++ {
		result = append(result, common.None)
	}
	return result
}

type Answers struct {
	defaultAnswer byte
	Answers       map[int]byte // flip index -> answer
	Presences     map[int]bool // flip index -> answer is present
}

func (answers Answers) Get(count int) []byte {
	var result []byte
	for i := 0; i < count; i++ {
		var answer byte
		if answers.Presences[int(i)] {
			answer = answers.Answers[int(i)]
		} else {
			answer = answers.defaultAnswer
		}
		result = append(result, answer)
	}
	return result
}
