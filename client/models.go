package client

import (
	"github.com/idena-network/idena-go/common/hexutil"
	"github.com/shopspring/decimal"
	"time"
)

type request struct {
	Method  string      `json:"method"`
	Version string      `json:"jsonrpc"`
	Id      int         `json:"id,omitempty"`
	Payload interface{} `json:"params,omitempty"`
}

type sendTxArgs struct {
	From       string  `json:"from"`
	To         *string `json:"to"`
	Amount     float32 `json:"amount"`
	MaxFee     float32 `json:"maxFee"`
	Type       uint16  `json:"type"`
	PayloadHex *string `json:"payload"`
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

type flipSubmitArgs struct {
	Hex  string `json:"hex"`
	Pair uint8  `json:"pairId"`
}

type changeProfileArgs struct {
	Banner   *hexutil.Bytes `json:"banner"`
	Nickname *string        `json:"nickname"`
}

type FlipAnswer struct {
	Answer     byte   `json:"answer"`
	Hash       string `json:"hash"`
	WrongWords bool   `json:"wrongWords"`
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
	Online        bool     `json:"online"`
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
	Extra bool   `json:"extra"`
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

type Peer struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	RemoteAddr string `json:"addr"`
}

type BurntCoins struct {
	Address string          `json:"address"`
	Amount  decimal.Decimal `json:"amount"`
}

type ChangeProfileResponse struct {
	TxHash string `json:"txHash"`
	Hash   string `json:"hash"`
}

type ProfileResponse struct {
	Nickname string `json:"nickname"`
	Banner   string `json:"banner"`
}
