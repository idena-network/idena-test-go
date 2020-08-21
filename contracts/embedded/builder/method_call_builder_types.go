package builder

import (
	"github.com/idena-network/idena-go/api"
	"github.com/idena-network/idena-go/common/hexutil"
)

type Call struct {
	MethodEstimate         string
	HandleEstimateResponse func(txReceipt *api.TxReceipt) error
	Method                 string
	HandleResponse         func(txHash string) error
	Payload                interface{}
}

type deployContractParams struct {
	ContractId  int
	From        string
	CodeHash    hexutil.Bytes
	Amount      string
	DynamicArgs interface{}
	MaxFee      string
}

type factEvidenceDeployContractDynamicArgs struct {
	Cid                  string
	StartTimeSecOffset   int
	VotingDuration       *int64
	PublicVotingDuration *int64
	WinnerThreshold      *int64
	Quorum               *int64
	CommitteeSize        *int64
	MaxOptions           *int64
	VotingMinPayment     *string
}

type callContractParams struct {
	ContractId  int
	From        string
	Method      string
	Amount      string
	DynamicArgs interface{}
	MaxFee      string
}
