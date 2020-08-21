package builder

import (
	"encoding/json"
	"fmt"
	"github.com/idena-network/idena-go/api"
	"github.com/idena-network/idena-go/common"
	"github.com/idena-network/idena-go/common/hexutil"
	"github.com/idena-network/idena-go/vm/embedded"
	"github.com/idena-network/idena-test-go/contracts/embedded/context"
	"github.com/idena-network/idena-test-go/log"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"strconv"
	"time"
)

func buildDeployContractCall(scenarioParams interface{}, contractsContext *context.ContractsContext) (*Call, error) {
	id, params, err := buildDeployContractParams(scenarioParams, contractsContext)
	if err != nil {
		return nil, err
	}
	return &Call{
		MethodEstimate: methodEstimateDeployContract,
		Method:         methodDeployContract,
		Payload:        []api.DeployContractArgs{*params},
		HandleEstimateResponse: func(txReceipt *api.TxReceipt) error {
			if !txReceipt.Success {
				return errors.Wrapf(err, "estimateDeployContract error: %v", txReceipt.Error)
			}
			contractsContext.SetContractAddress(id, txReceipt.Contract)
			return nil
		},
		HandleResponse: func(txHash string) error {
			log.Info(fmt.Sprintf("tx: %v", txHash)) // todo tmp
			return nil
		},
	}, nil
}

func buildDeployContractParams(scenarioParams interface{}, contractsContext *context.ContractsContext) (int, *api.DeployContractArgs, error) {
	params := &deployContractParams{}
	b, _ := json.Marshal(scenarioParams)
	_ = json.Unmarshal(b, params)

	contractsContext.SetContractCode(params.ContractId, params.CodeHash.String())

	codeHash, err := buildEmbeddedContractType(params.CodeHash)
	if err != nil {
		return 0, nil, err
	}

	var amount decimal.Decimal
	if len(params.Amount) > 0 {
		amount, err = decimal.NewFromString(params.Amount)
		if err != nil {
			return 0, nil, errors.Wrapf(err, "unable to parse decimal %v", params.Amount)
		}
	}

	var maxFee decimal.Decimal
	if len(params.MaxFee) > 0 {
		maxFee, err = decimal.NewFromString(params.MaxFee)
		if err != nil {
			return 0, nil, errors.Wrapf(err, "unable to parse decimal %v", params.MaxFee)
		}
	}

	args, err := buildDeployContractDynamicArgs(params.CodeHash, params.DynamicArgs)
	if err != nil {
		return 0, nil, err
	}

	nodeParams := &api.DeployContractArgs{
		From:     common.HexToAddress(params.From),
		CodeHash: codeHash.Bytes(),
		Amount:   amount,
		Args:     *args,
		MaxFee:   maxFee,
	}
	return params.ContractId, nodeParams, errors.Wrap(err, "unable to parse params")
}

func buildEmbeddedContractType(codeHash hexutil.Bytes) (embedded.EmbeddedContractType, error) {
	switch codeHash.String() {
	case codeHashFactEvidence:
		return embedded.FactEvidenceContract, nil
	default:
		return embedded.EmbeddedContractType{}, errors.Errorf("unsupported contract code hash %v", codeHash)
	}
}

func buildDeployContractDynamicArgs(codeHash hexutil.Bytes, scenarioDynamicArgs interface{}) (*api.DynamicArgs, error) {
	switch codeHash.String() {
	case codeHashFactEvidence:
		return buildFactEvidenceDeployContractDynamicArgs(scenarioDynamicArgs)
	default:
		return nil, errors.Errorf("unsupported contract code hash %v", codeHash)
	}
}

func buildFactEvidenceDeployContractDynamicArgs(scenarioDynamicArgs interface{}) (*api.DynamicArgs, error) {
	dynamicArgs := &factEvidenceDeployContractDynamicArgs{}
	b, _ := json.Marshal(scenarioDynamicArgs)
	_ = json.Unmarshal(b, dynamicArgs)
	nodeCid, err := cidStrToHex(dynamicArgs.Cid)
	if err != nil {
		return nil, err
	}
	startTime := time.Now().Add(time.Second * time.Duration(dynamicArgs.StartTimeSecOffset))
	nodeDynamicArgs := api.DynamicArgs{
		&api.DynamicArg{
			Index:  0,
			Format: "hex",
			Value:  nodeCid,
		},
		&api.DynamicArg{
			Index:  1,
			Format: "uint64",
			Value:  strconv.FormatUint(uint64(startTime.Unix()), 10),
		},
	}
	if dynamicArgs.VotingDuration != nil {
		nodeDynamicArgs = append(nodeDynamicArgs, &api.DynamicArg{
			Index:  2,
			Format: "uint64",
			Value:  strconv.FormatInt(*dynamicArgs.VotingDuration, 10),
		})
	}
	if dynamicArgs.PublicVotingDuration != nil {
		nodeDynamicArgs = append(nodeDynamicArgs, &api.DynamicArg{
			Index:  3,
			Format: "uint64",
			Value:  strconv.FormatInt(*dynamicArgs.PublicVotingDuration, 10),
		})
	}
	if dynamicArgs.WinnerThreshold != nil {
		nodeDynamicArgs = append(nodeDynamicArgs, &api.DynamicArg{
			Index:  4,
			Format: "uint64",
			Value:  strconv.FormatInt(*dynamicArgs.WinnerThreshold, 10),
		})
	}
	if dynamicArgs.Quorum != nil {
		nodeDynamicArgs = append(nodeDynamicArgs, &api.DynamicArg{
			Index:  5,
			Format: "uint64",
			Value:  strconv.FormatInt(*dynamicArgs.Quorum, 10),
		})
	}
	if dynamicArgs.CommitteeSize != nil {
		nodeDynamicArgs = append(nodeDynamicArgs, &api.DynamicArg{
			Index:  6,
			Format: "uint64",
			Value:  strconv.FormatInt(*dynamicArgs.CommitteeSize, 10),
		})
	}
	if dynamicArgs.MaxOptions != nil {
		nodeDynamicArgs = append(nodeDynamicArgs, &api.DynamicArg{
			Index:  7,
			Format: "uint64",
			Value:  strconv.FormatInt(*dynamicArgs.MaxOptions, 10),
		})
	}
	if dynamicArgs.VotingMinPayment != nil {
		votingMinPayment, err := stringToBigIntHex(*dynamicArgs.VotingMinPayment)
		if err != nil {
			return nil, err
		}
		nodeDynamicArgs = append(nodeDynamicArgs, &api.DynamicArg{
			Index:  8,
			Format: "bigint",
			Value:  votingMinPayment,
		})
	}
	return &nodeDynamicArgs, nil
}
