package builder

import (
	"github.com/idena-network/idena-go/common/hexutil"
	"github.com/idena-network/idena-test-go/contracts/embedded/context"
	"github.com/idena-network/idena-test-go/scenario"
	"github.com/ipfs/go-cid"
	"github.com/pkg/errors"
	"math/big"
)

const (
	methodEstimateDeployContract = "dna_estimateDeployContract"
	methodDeployContract         = "dna_deployContract"
	methodEstimateCallContract   = "dna_estimateCallContract"
	methodCallContract           = "dna_callContract"

	codeHashFactEvidence = "0x02"
)

func BuildCall(scenarioCall scenario.ContractMethodCall, contractsContext *context.ContractsContext) (*Call, error) {
	switch scenarioCall.Method {
	case methodDeployContract:
		return buildDeployContractCall(scenarioCall.Params, contractsContext)
	case methodCallContract:
		return buildCallContractCall(scenarioCall.Params, contractsContext)
	default:
		return nil, errors.Errorf("unsupported method %v", scenarioCall.Method)
	}
}

func cidStrToHex(cidStr string) (string, error) {
	c, err := cid.Parse(cidStr)
	if err != nil {
		return "", errors.Wrapf(err, "unable to parse cid %v", cidStr)
	}
	return hexutil.Encode(c.Bytes()), nil
}

func stringToBigIntHex(value string) (string, error) {
	bigInt, ok := new(big.Int).SetString(value, 10)
	if !ok {
		return "", errors.Errorf("unable to pars big int %v", value)
	}
	return hexutil.Encode(bigInt.Bytes()), nil
}
