package builder

import (
	"encoding/json"
	"fmt"
	"github.com/idena-network/idena-go/api"
	"github.com/idena-network/idena-go/common"
	"github.com/idena-network/idena-test-go/contracts/embedded/context"
	"github.com/idena-network/idena-test-go/log"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
)

const (
	MethodStartVoting = "startVoting"
)

func buildCallContractCall(scenarioParams interface{}, contractsContext *context.ContractsContext) (*Call, error) {
	params, err := buildCallContractParams(scenarioParams, contractsContext)
	if err != nil {
		return nil, err
	}
	return &Call{
		MethodEstimate: methodEstimateCallContract,
		Method:         methodCallContract,
		Payload:        []api.CallContractArgs{*params},
		HandleEstimateResponse: func(txReceipt *api.TxReceipt) error {
			if !txReceipt.Success {
				return errors.Wrapf(err, "estimateCallContract error: %v", txReceipt.Error)
			}
			return nil
		},
		HandleResponse: func(txHash string) error {
			log.Info(fmt.Sprintf("tx: %v", txHash)) // todo tmp
			return nil
		},
	}, nil
}

func buildCallContractParams(scenarioParams interface{}, contractsContext *context.ContractsContext) (*api.CallContractArgs, error) {
	params := &callContractParams{}
	b, _ := json.Marshal(scenarioParams)
	_ = json.Unmarshal(b, params)

	var amount decimal.Decimal
	if len(params.Amount) > 0 {
		var err error
		amount, err = decimal.NewFromString(params.Amount)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to parse decimal %v", params.Amount)
		}
	}

	var maxFee decimal.Decimal
	if len(params.MaxFee) > 0 {
		var err error
		maxFee, err = decimal.NewFromString(params.MaxFee)
		if err != nil {
			return nil, errors.Wrapf(err, "unable to parse decimal %v", params.MaxFee)
		}
	}

	codeHash, _ := contractsContext.GetContractCodeById(params.ContractId)
	args, err := buildCallContractDynamicArgs(codeHash, params.Method, params.DynamicArgs)
	if err != nil {
		return nil, err
	}

	contractAddress, _ := contractsContext.GetContractAddressById(params.ContractId)

	nodeParams := &api.CallContractArgs{
		From:     common.HexToAddress(params.From),
		Contract: contractAddress,
		Method:   params.Method,
		Amount:   amount,
		Args:     *args,
		MaxFee:   maxFee,
	}
	return nodeParams, errors.Wrap(err, "unable to parse params")
}

func buildCallContractDynamicArgs(codeHash string, method string, scenarioDynamicArgs interface{}) (*api.DynamicArgs, error) {
	switch codeHash {
	case codeHashFactEvidence:
		return buildFactEvidenceCallContractDynamicArgs(method, scenarioDynamicArgs)
	default:
		return nil, errors.Errorf("unsupported contract code hash %v", codeHash)
	}
}

func buildFactEvidenceCallContractDynamicArgs(method string, scenarioDynamicArgs interface{}) (*api.DynamicArgs, error) {
	switch method {
	case MethodStartVoting:
		return buildFactEvidenceCallContractStartVotingDynamicArgs(scenarioDynamicArgs)
	default:
		return nil, errors.Errorf("unsupported contract method %v", method)
	}
}

func buildFactEvidenceCallContractStartVotingDynamicArgs(scenarioDynamicArgs interface{}) (*api.DynamicArgs, error) {
	return &api.DynamicArgs{}, nil
}
