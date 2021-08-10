package client

import (
	"github.com/idena-network/idena-go/common/hexutil"
	"github.com/shopspring/decimal"
	"time"
)

type request struct {
	Key     string      `json:"key"`
	Method  string      `json:"method"`
	Version string      `json:"jsonrpc"`
	Id      int         `json:"id,omitempty"`
	Payload interface{} `json:"params,omitempty"`
}

type sendTxArgs struct {
	From       string   `json:"from"`
	To         *string  `json:"to"`
	Amount     float32  `json:"amount"`
	MaxFee     *float32 `json:"maxFee"`
	Type       uint16   `json:"type"`
	PayloadHex *string  `json:"payload"`
	UseProto   bool     `json:"useProto"`
}

type storeToIpfsTxArgs struct {
	Cid string `json:"cid"`
}

type sendInviteArgs struct {
	To     string  `json:"to"`
	Amount float32 `json:"amount"`
}

type burnArgs struct {
	From   string  `json:"from"`
	Amount float32 `json:"amount"`
	MaxFee float32 `json:"maxFee"`
	Key    string  `json:"key"`
}

type submitAnswersArgs struct {
	Answers []FlipAnswer `json:"answers"`
}

type flipSubmitArgs struct {
	PrivateHex string `json:"privateHex"`
	PublicHex  string `json:"publicHex"`
	Pair       uint8  `json:"pairId"`
}

type changeProfileArgs struct {
	Info     *hexutil.Bytes `json:"info"`
	Nickname *string        `json:"nickname"`
}

type FlipAnswer struct {
	Answer byte   `json:"answer"`
	Hash   string `json:"hash"`
	Grade  byte   `json:"grade"`
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
	PublicHex  string `json:"hex"`
	PrivateHex string `json:"privateHex"`
}

type SubmitAnswersResponse struct {
	TxHash string `json:"txHash"`
}

type CeremonyIntervals struct {
	FlipLotteryDuration  float64
	ShortSessionDuration float64
	LongSessionDuration  float64
}

type Peer struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	RemoteAddr string `json:"addr"`
}

type BurntCoins struct {
	Address string          `json:"address"`
	Amount  decimal.Decimal `json:"amount"`
	Key     string          `json:"key"`
}

type ChangeProfileResponse struct {
	TxHash string `json:"txHash"`
	Hash   string `json:"hash"`
}

type ProfileResponse struct {
	Nickname string `json:"nickname"`
	Info     string `json:"info"`
}

type FlipResponse2 struct {
	PublicHex  hexutil.Bytes `json:"publicHex"`
	PrivateHex hexutil.Bytes `json:"privateHex"`
}

type FlipKeysResponse struct {
	PublicKey  hexutil.Bytes `json:"publicKey"`
	PrivateKey hexutil.Bytes `json:"privateKey"`
}
