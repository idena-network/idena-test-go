package scenario

const correctAnswer = byte(1) // fixme currently we consider Left answer as correct
const wrongAnswer = byte(2)   // fixme currently we consider Right answer as wrong

type Scenario struct {
	EpochNewUsers     map[int]int   // Epoch -> new users count
	EpochNodeStarts   map[int][]int //Epoch -> nodes to start
	EpochNodeStops    map[int][]int //Epoch -> nodes to stop
	CeremonyMinOffset int
	Ceremonies        map[int]*Ceremony // Epoch -> Ceremony
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
}

type AnswerRates struct {
	Wrong         float32
	None          float32
	Inappropriate float32
}

func (answerRates AnswerRates) Get(count int) []byte {
	wrongCount := int(float32(count)*answerRates.Wrong + 0.5)
	noneCount := int(float32(count)*answerRates.None + 0.5)
	inappropriateCount := int(float32(count)*answerRates.Inappropriate + 0.5)
	correctCount := count - wrongCount - noneCount - inappropriateCount
	var result []byte
	for i := 0; i < correctCount; i++ {
		result = append(result, correctAnswer)
	}
	for i := 0; i < wrongCount; i++ {
		result = append(result, wrongAnswer)
	}
	for i := 0; i < inappropriateCount; i++ {
		result = append(result, 3)
	}
	for i := 0; i < noneCount; i++ {
		result = append(result, 0)
	}
	return result
}

type Answers struct {
	Answers   map[int]byte // flip index -> answer
	Presences map[int]bool // flip index -> answer is present
}

func (answers Answers) Get(count int) []byte {
	var result []byte
	for i := 0; i < count; i++ {
		var answer byte
		if answers.Presences[int(i)] {
			answer = answers.Answers[int(i)]
		} else {
			answer = correctAnswer
		}
		result = append(result, answer)
	}
	return result
}
