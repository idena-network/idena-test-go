package scenario

type incomingScenario struct {
	Users             int            `json:"users"` // users count
	CeremonyMinOffset int            `json:"ceremonyMinOffset"`
	NewUsers          []newUsers     `json:"newUsers"` // users to create later
	NodeStarts        []epochsNodes  `json:"nodeStarts"`
	NodeStops         []epochsNodes  `json:"nodeStops"`
	NodeOnlines       []epochsNodes  `json:"nodeOnlines"`
	NodeOfflines      []epochsNodes  `json:"nodeOfflines"`
	Txs               []transactions `json:"txs"`
	DefaultAnswer     byte           `json:"defaultAnswer"`
	Ceremonies        []ceremony     `json:"ceremonies"`
}

type newUsers struct {
	Epochs string `json:"epochs"` // example: "1,3-5,8" means 1,3,4,5,8
	Count  int    `json:"count"`
}

type epochsNodes struct {
	Epochs string `json:"epochs"` // "1,3-5,8" means 1,3,4,5,8
	Nodes  string `json:"nodes"`  // "1,3-5,8" means 1,3,4,5,8
}

type transactions struct {
	epochsNodes
	PeriodMs int64
}

type ceremony struct {
	Epochs         string         `json:"epochs"` // "1,3-5,8" means 1,3,4,5,8
	UserCeremonies []userCeremony `json:"userCeremonies"`
	Assertion      *assertion     `json:"assertion"`
}

type userCeremony struct {
	Users            string       `json:"users"` // "1,3-5,8" means 1,3,4,5,8
	SubmitFlips      int          `json:"submitFlips"`
	ShortAnswers     []answer     `json:"shortAnswers"`
	ShortAnswerRates *answerRates `json:"shortAnswerRates"`
	LongAnswers      []answer     `json:"longAnswers"`
	LongAnswerRates  *answerRates `json:"longAnswerRates"`
}

type answerRates struct {
	Left          float32 `json:"left"`
	Right         float32 `json:"right"`
	Inappropriate float32 `json:"inappropriate"`
}

type answer struct {
	Index  int  `json:"index"`
	Answer byte `json:"answer"`
}

type assertion struct {
	States []stateAssertion `json:"states"`
	Nodes  []nodeAssertion  `json:"nodes"`
}

type stateAssertion struct {
	State string `json:"state"`
	Count int    `json:"count"`
}

type nodeAssertion struct {
	Users            string `json:"users"` // "1,3-5,8" means 1,3,4,5,8
	MadeFlips        int    `json:"madeFlips"`
	RequiredFlips    int    `json:"requiredFlips"`
	AvailableInvites int    `json:"availableInvites"`
	State            string `json:"state"`
	Online           bool   `json:"online"`
}
