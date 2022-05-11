package scenario

type incomingScenario struct {
	Users              int                  `json:"users"` // users count
	InviteAmount       *float32             `json:"inviteAmount"`
	CeremonyMinOffset  int                  `json:"ceremonyMinOffset"`
	NewUsers           []newUsers           `json:"newUsers"`           // users to create later
	NewUsersAfterFlips []newUsers           `json:"newUsersAfterFlips"` // users to create later and after flips are submitted
	NodeStarts         []delayedEpochsNodes `json:"nodeStarts"`
	NodeStops          []delayedEpochsNodes `json:"nodeStops"`
	NodeOnlines        []epochsNodes        `json:"nodeOnlines"`
	NodeOfflines       []epochsNodes        `json:"nodeOfflines"`
	DelayedKeys        []epochsNodes        `json:"delayedKeys"`
	Txs                []transactions       `json:"txs"`
	DefaultAnswer      byte                 `json:"defaultAnswer"`
	Ceremonies         []ceremony           `json:"ceremonies"`
	NodeUpdates        []nodeUpdates        `json:"nodeUpdates"`
	Delegations        []delegations        `json:"delegations"`
	KillDelegators     []killDelegators     `json:"killDelegators"`
	KillInvitees       []killInvitees       `json:"killInvitees"`
	Undelegations      []epochsNodes        `json:"undelegations"`
	MultiBotPools      *multiBotPools       `json:"multiBotPools"`
	StoreToIpfsTxs     []storeToIpfsTxs     `json:"storeToIpfsTxs"`
	Kills              []epochsNodes        `json:"kills"`
	AddStakes          []addStakes          `json:"addStakes"`
}

type newUsers struct {
	Epochs       string   `json:"epochs"` // example: "1,3-5,8" means 1,3,4,5,8
	Count        int      `json:"count"`
	Inviter      *int     `json:"inviter"`
	Command      string   `json:"command"`
	SharedNode   *int     `json:"sharedNode"`
	InviteAmount *float32 `json:"inviteAmount"`
}

type epochsNodes struct {
	Epochs      string `json:"epochs"` // "1,3-5,8" means 1,3,4,5,8
	Nodes       string `json:"nodes"`  // "1,3-5,8" means 1,3,4,5,8
	OnlyGodMode bool   `json:"onlyGodMode"`
}

type delayedEpochsNodes struct {
	epochsNodes
	DelaySec int64 `json:"delaySec"`
}

type transactions struct {
	epochsNodes
	PeriodMs int64 `json:"periodMs"`
}

type ceremony struct {
	Epochs         string         `json:"epochs"` // "1,3-5,8" means 1,3,4,5,8
	UserCeremonies []userCeremony `json:"userCeremonies"`
	Assertion      *assertion     `json:"assertion"`
}

type userCeremony struct {
	Users            string `json:"users"` // "1,3-5,8" means 1,3,4,5,8
	SubmitFlips      *int   `json:"submitFlips"`
	SkipValidation   bool   `json:"skipValidation"`
	FailShortSession bool   `json:"failShortSession"`
	// todo remove deprecated fields
	ShortAnswers     []answer     `json:"shortAnswers"`     // deprecated
	ShortAnswerRates *answerRates `json:"shortAnswerRates"` // deprecated
	LongAnswers      []answer     `json:"longAnswers"`      // deprecated
	LongAnswerRates  *answerRates `json:"longAnswerRates"`  // deprecated
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
	Users            string  `json:"users"` // "1,3-5,8" means 1,3,4,5,8
	MadeFlips        *int    `json:"madeFlips"`
	RequiredFlips    *int    `json:"requiredFlips"`
	AvailableInvites *int    `json:"availableInvites"`
	State            *string `json:"state"`
	Online           *bool   `json:"online"`
}

type nodeUpdates struct {
	delayedEpochsNodes
	StepDelaySec int64  `json:"stepDelaySec"`
	Command      string `json:"command"`
}

type delegations struct {
	epochsNodes
	Delegatee int `json:"delegatee"`
}

type addStakes struct {
	epochsNodes
	Amount float32 `json:"amount"`
}

type multiBotPools struct {
	Sizes             []int   `json:"sizes"`
	BotDelegatorsRate float64 `json:"botDelegatorsRate"`
}

type killDelegators struct {
	epochsNodes
	Delegatee int `json:"delegatee"`
}

type killInvitees struct {
	epochsNodes
	Inviter int `json:"inviter"`
}

type storeToIpfsTxs struct {
	delayedEpochsNodes
	Sizes        []int `json:"sizes"`
	StepDelaySec int64 `json:"stepDelaySec"`
}
