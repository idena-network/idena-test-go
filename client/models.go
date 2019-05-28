package client

import (
	"time"
)

type request struct {
	Method  string      `json:"method"`
	Version string      `json:"jsonrpc"`
	Id      int         `json:"id,omitempty"`
	Payload interface{} `json:"params,omitempty"`
}

type sendInviteArgs struct {
	To string `json:"to"`
}

type activateInviteArgs struct {
	To string `json:"to"`
}

type submitAnswersArgs struct {
	Answers []FlipAnswer `json:"answers"`
}

type FlipAnswer struct {
	Easy   bool `json:"easy"`
	Answer byte `json:"answer"`
}

type response struct {
	Version string      `json:"jsonrpc"`
	Id      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result"`
	Error   *Error      `json:"error"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Epoch struct {
	Epoch          uint16    `json:"epoch"`
	NextValidation time.Time `json:"nextValidation"`
	CurrentPeriod  string    `json:"currentPeriod"`
}

type Identity struct {
	Address       string   `json:"address"`
	State         string   `json:"state"`
	RequiredFlips int      `json:"requiredFlips"`
	Flips         []string `json:"flips"`
	Invites       int      `json:"invites"`
}

type Invite struct {
	Hash     string `json:"hash"`
	Receiver string `json:"receiver"`
	Key      string `json:"key"`
}

type FlipSubmitResponse struct {
	TxHash string `json:"txHash"`
	Hash   string `json:"hash"`
}

type FlipHashesResponse struct {
	Hash  string `json:"hash"`
	Ready bool   `json:"ready"`
}

type FlipResponse struct {
	Hex string `json:"hex"`
}

type SubmitAnswersResponse struct {
	TxHash string `json:"txHash"`
}

type CeremonyIntervals struct {
	ValidationInterval       float64
	FlipLotteryDuration      float64
	ShortSessionDuration     float64
	LongSessionDuration      float64
	AfterLongSessionDuration float64
}
