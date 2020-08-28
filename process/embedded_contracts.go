package process

import (
	"fmt"
	"github.com/google/tink/go/subtle/random"
	"github.com/idena-network/idena-go/api"
	"github.com/idena-network/idena-go/common"
	"github.com/idena-network/idena-go/common/hexutil"
	"github.com/idena-network/idena-test-go/client"
	"github.com/idena-network/idena-test-go/contracts/embedded/builder"
	"github.com/idena-network/idena-test-go/contracts/embedded/context"
	"github.com/idena-network/idena-test-go/log"
	"github.com/idena-network/idena-test-go/scenario"
	"github.com/idena-network/idena-test-go/user"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"strconv"
	"sync"
	"time"
)

const (
	ErrorMessageInvalidProof     = "invalid proof"
	ErrorMessageWrongBlockNumber = "wrong block number"
	ErrorMessageWrongVoteHash    = "wrong vote hash"
)

type UserVotingContext struct {
	Proof            string
	Salt             hexutil.Bytes
	Vote             byte
	VoteHash         string
	VotingMinPayment string
	VoteBlock        uint64
}

func (process *Process) tmp() {
	contractsContext := context.NewContractsContext()
	contractOwner := process.users[0]

	if err := deployVoting(contractOwner, process.sc.Contracts.Calls[0], contractsContext); err != nil {
		process.handleError(err, "unable to deploy contract")
	}

	// Send coins to contract
	contractAddress, _ := contractsContext.GetContractAddressById(55)
	amount, _ := strconv.ParseFloat(process.sc.Contracts.Amount, 32)
	if err := sendCoinsToContract(contractOwner, contractAddress, float32(amount)); err != nil {
		process.handleError(err, "unable to deploy contract")
	}

	if err := startVoting(contractOwner, process.sc.Contracts.Calls[1], contractsContext); err != nil {
		process.handleError(err, "unable to start voting")
	}
	if err := performUsersVoting(process.users, process.handleError, contractsContext); err != nil {
		process.handleError(err, "unable to perform voting")
	}

	time.Sleep(time.Hour * 24)
}

func deployVoting(u *user.User, scenario scenario.ContractMethodCall, contractsContext *context.ContractsContext) error {
	call, err := builder.BuildCall(scenario, contractsContext)
	if err != nil {
		return errors.Wrap(err, "unable to build call")
	}

	txReceipt, err := u.Client.ContractCallEstimate(call.MethodEstimate, call.Payload)
	if err == nil && !txReceipt.Success {
		err = errors.New(txReceipt.Error)
	}
	if err != nil {
		return errors.Wrap(err, "failed estimate method call")
	}
	log.Info(fmt.Sprintf("Successfully sent estimate: %v", txReceipt))
	if err := call.HandleEstimateResponse(txReceipt); err != nil {
		return errors.Wrap(err, "unable to handle estimate response")
	}

	txHash, err := u.Client.ContractCall(call.Method, call.Payload)
	if err != nil {
		return errors.Wrap(err, "unable to send")
	}
	log.Info(fmt.Sprintf("Successfully sent: %v", txHash))
	if err := call.HandleResponse(txHash); err != nil {
		return errors.Wrap(err, "unable to handle response")
	}

	err = waitForMinedTransaction(u.Client, txHash, time.Minute)
	if err != nil {
		return err
	}

	return nil
}

func sendCoinsToContract(u *user.User, contractAddress common.Address, amount float32) error {
	log.Info(fmt.Sprintf("Sending %v coins to contract address", amount))
	contractAddressHex := contractAddress.Hex()
	txHash, err := u.Client.SendTransaction(0, u.Address, &contractAddressHex, amount, 9999, nil)
	if err != nil {
		return errors.Wrap(err, "unable to send")
	}
	err = waitForMinedTransaction(u.Client, txHash, time.Minute)
	if err != nil {
		return err
	}
	log.Info("Sent coins to contract address")
	return nil
}

func startVoting(u *user.User, scenario scenario.ContractMethodCall, contractsContext *context.ContractsContext) error {
	call, err := builder.BuildCall(scenario, contractsContext)
	if err != nil {
		return errors.Wrap(err, "unable to build call")
	}

	txReceipt, err := u.Client.ContractCallEstimate(call.MethodEstimate, call.Payload)
	if err == nil && !txReceipt.Success {
		err = errors.New(txReceipt.Error)
	}
	if err != nil {
		return errors.Wrap(err, "failed estimate method call")
	}
	log.Info(fmt.Sprintf("Successfully sent estimate: %v", txReceipt))
	if err := call.HandleEstimateResponse(txReceipt); err != nil {
		return errors.Wrap(err, "unable to handle estimate response")
	}

	txHash, err := u.Client.ContractCall(call.Method, call.Payload)
	if err != nil {
		return errors.Wrap(err, "unable to send")
	}
	log.Info(fmt.Sprintf("Successfully sent: %v", txHash))
	if err := call.HandleResponse(txHash); err != nil {
		return errors.Wrap(err, "unable to handle response")
	}

	err = waitForMinedTransaction(u.Client, txHash, time.Minute)
	if err != nil {
		return err
	}
	return nil
}

func finishVoting(u *user.User, contractAddress common.Address) error {
	payload := []api.CallContractArgs{
		{
			Contract: contractAddress,
			Method:   "finishVoting",
			MaxFee:   decimal.RequireFromString("9999"), // todo
		},
	}
	txReceipt, err := u.Client.ContractCallEstimate("dna_estimateCallContract", payload)
	if err == nil && !txReceipt.Success {
		err = errors.New(txReceipt.Error)
	}
	if err != nil {
		return errors.Wrap(err, "failed estimate method call")
	}

	txHash, err := u.Client.ContractCall("dna_callContract", payload)
	if err != nil {
		return errors.Wrap(err, "failed method call")
	}
	if err := waitForMinedTransaction(u.Client, txHash, time.Minute); err != nil {
		return err
	}
	return nil
}

func prolongVoting() {

}

func terminateContract(u *user.User, contractAddress common.Address) error {
	payload := []api.CallContractArgs{
		{
			Contract: contractAddress,
			Method:   "terminate",
			MaxFee:   decimal.RequireFromString("9999"), // todo
		},
	}
	txReceipt, err := u.Client.ContractCallEstimate("dna_estimateCallContract", payload)
	if err == nil && !txReceipt.Success {
		err = errors.New(txReceipt.Error)
	}
	if err != nil {
		return errors.Wrap(err, "failed estimate method call")
	}

	txHash, err := u.Client.ContractCall("dna_callContract", payload)
	if err != nil {
		return errors.Wrap(err, "failed method call")
	}
	if err := waitForMinedTransaction(u.Client, txHash, time.Minute); err != nil {
		return err
	}
	return nil
}

func performUsersVoting(users []*user.User, handleError func(error, string), contractsContext *context.ContractsContext) error {
	finishVotingWG := &sync.WaitGroup{}
	finishVotingWG.Add(len(users))
	for _, u := range users {
		go performUserVoting(u, handleError, contractsContext, finishVotingWG)
	}
	return nil
}

func performUserVoting(u *user.User, handleError func(error, string), contractsContext *context.ContractsContext, finishVotingWG *sync.WaitGroup) {
	if err := u.WaitForSync(); err != nil {
		handleError(err, fmt.Sprintf("%v unable to sync", u.GetInfo()))
	}

	votingContext := &UserVotingContext{
		Salt: random.GetRandomBytes(20),
		Vote: 1, // todo
	}
	contractAddress, _ := contractsContext.GetContractAddressById(55) // todo id

	// Get proof
	proof, err := getProof(u, contractAddress)
	if err != nil {
		handleError(err, fmt.Sprintf("%v unable to get proof", u.GetInfo()))
	}
	if len(proof) == 0 {
		log.Info(fmt.Sprintf("%v user is not in committee", u.GetInfo()))
	} else {
		votingContext.Proof = proof
		log.Info(fmt.Sprintf("%v user is in committee, proof: %v", u.GetInfo(), proof))
	}

	// Get vote hash
	voteHash, err := getVoteHash(u, votingContext.Vote, votingContext.Salt, contractAddress)
	if err != nil {
		handleError(err, fmt.Sprintf("%v unable to get vote hash", u.GetInfo()))
	}
	votingContext.VoteHash = voteHash
	log.Info(fmt.Sprintf("%v user is in committee, voteHash: %v", u.GetInfo(), voteHash))

	// Get voting min payment
	votingMinPayment, err := getVotingMinPayment(u, contractAddress)
	if err != nil {
		handleError(err, fmt.Sprintf("%v unable to get voting min payment", u.GetInfo()))
	}
	votingContext.VotingMinPayment = votingMinPayment
	log.Info(fmt.Sprintf("%v votingMinPayment: %v", u.GetInfo(), votingMinPayment))

	voteBlock, err := getVoteBlock(u, contractAddress)
	if err != nil {
		handleError(err, fmt.Sprintf("%v unable to get vote block", u.GetInfo()))
	}
	votingContext.VoteBlock = voteBlock
	log.Info(fmt.Sprintf("%v voteBlock: %v", u.GetInfo(), voteBlock))

	inCommittee := true
	// Send vote proof
	if err := sendVoteProof(u, contractAddress, votingContext.VotingMinPayment, votingContext.VoteHash, votingContext.Proof); err != nil {
		if errors.Cause(err).Error() == ErrorMessageInvalidProof && len(votingContext.Proof) == 0 {
			inCommittee = false
			log.Info(fmt.Sprintf("%v can't send vote proof as it is not in committee", u.GetInfo()))
		} else {
			handleError(err, fmt.Sprintf("%v unable to send vote proof", u.GetInfo()))
		}
	} else {
		log.Info(fmt.Sprintf("%v sent vote proof", u.GetInfo()))
	}

	// Send vote
	if inCommittee {
		if err := sendVote(u, contractAddress, votingContext.VoteBlock, votingContext.Vote, votingContext.Salt); err != nil {
			if errors.Cause(err).Error() == ErrorMessageWrongVoteHash && len(votingContext.Proof) == 0 {
				log.Info(fmt.Sprintf("%v can't send vote as it is not in committee", u.GetInfo()))
			} else {
				handleError(err, fmt.Sprintf("%v unable to send vote", u.GetInfo()))
			}
		} else {
			log.Info(fmt.Sprintf("%v sent vote", u.GetInfo()))
		}
	}

	if err := waitForBlock(u.Client, voteBlock+3, time.Minute*10); err != nil {
		handleError(err, fmt.Sprintf("%v error while waiting for block", u.GetInfo()))
	}

	finishVotingWG.Done()

	// todo
	if u.Index == 1 {
		finishVotingWG.Wait()
		if err := finishVoting(u, contractAddress); err != nil {
			handleError(err, fmt.Sprintf("%v unable to finish voting", u.GetInfo()))
		}
		log.Info(fmt.Sprintf("%v finished voting", u.GetInfo()))

		//if err := terminateContract(u, contractAddress); err != nil {
		//	handleError(err, fmt.Sprintf("%v unable to terminate contract", u.GetInfo()))
		//}
		//log.Info(fmt.Sprintf("%v terminated contract", u.GetInfo()))
	}
}

func getProof(u *user.User, contractAddress common.Address) (string, error) {
	var res string
	err := u.Client.ReadonlyCallContract([]api.ReadonlyCallContractArgs{
		{
			Method:   "proof",
			Contract: contractAddress,
			Format:   "hex",
		},
	}, &res)
	if err != nil {
		if err.Error() == ErrorMessageInvalidProof {
			return "", nil
		}
		return "", errors.Wrap(err, "unable to call method proof")
	}
	return res, nil
}

func getVoteHash(u *user.User, vote byte, salt hexutil.Bytes, contractAddress common.Address) (string, error) {
	var res string
	err := u.Client.ReadonlyCallContract([]api.ReadonlyCallContractArgs{
		{
			Method:   "voteHash",
			Contract: contractAddress,
			Format:   "hex",
			Args: api.DynamicArgs{
				{
					Index:  0,
					Format: "byte",
					Value:  strconv.FormatUint(uint64(vote), 10),
				},
				{
					Index:  1,
					Format: "hex",
					Value:  salt.String(),
				},
			},
		},
	}, &res)
	if err != nil {
		return "", errors.Wrap(err, "unable to call method voteHash")
	}
	return res, nil
}

func getVotingMinPayment(u *user.User, contractAddress common.Address) (string, error) {
	var res string
	err := u.Client.ReadContractData(contractAddress, "votingMinPayment", "dna", &res)
	if err != nil {
		return "", errors.Wrap(err, "unable to get votingMinPayment")
	}
	return res, nil
}

func getVoteBlock(u *user.User, contractAddress common.Address) (uint64, error) {
	var res uint64
	err := u.Client.ReadonlyCallContract([]api.ReadonlyCallContractArgs{
		{
			Method:   "voteBlock",
			Contract: contractAddress,
			Format:   "uint64",
		},
	}, &res)
	if err != nil {
		return 0, errors.Wrap(err, "unable to call method voteBlock")
	}
	return res, nil
}

func sendVoteProof(u *user.User, contractAddress common.Address, votingMinPayment, voteHash, proof string) error {
	payload := []api.CallContractArgs{
		{
			Contract: contractAddress,
			Amount:   decimal.RequireFromString(votingMinPayment),
			Method:   "sendVoteProof",
			MaxFee:   decimal.RequireFromString("9999"), // todo
			Args: api.DynamicArgs{
				{
					Index:  0,
					Format: "hex",
					Value:  voteHash,
				},
				{
					Index:  1,
					Format: "hex",
					Value:  proof,
				},
			},
		},
	}
	txReceipt, err := u.Client.ContractCallEstimate("dna_estimateCallContract", payload)
	if err == nil && !txReceipt.Success {
		err = errors.New(txReceipt.Error)
	}
	if err != nil {
		return errors.Wrap(err, "failed estimate method call")
	}

	txHash, err := u.Client.ContractCall("dna_callContract", payload)
	if err != nil {
		return errors.Wrap(err, "unable to send vote proof")
	}
	if err := waitForMinedTransaction(u.Client, txHash, time.Minute); err != nil {
		return err
	}
	return nil
}

func sendVote(u *user.User, contractAddress common.Address, voteBlock uint64, vote byte, salt hexutil.Bytes) error {
	payload := []api.CallContractArgs{
		{
			Contract:       contractAddress,
			Method:         "sendVote",
			MaxFee:         decimal.RequireFromString("9999"), // todo
			BroadcastBlock: voteBlock,
			Args: api.DynamicArgs{
				{
					Index:  0,
					Format: "byte",
					Value:  strconv.FormatUint(uint64(vote), 10),
				},
				{
					Index:  1,
					Format: "hex",
					Value:  salt.String(),
				},
			},
		},
	}
	//txReceipt, err := u.Client.ContractCallEstimate("dna_estimateCallContract", payload)

	// Wait for time to send vote
	//first := true
	//for err == nil && !txReceipt.Success && txReceipt.Error == ErrorMessageWrongBlockNumber {
	//	if first {
	//		log.Info(fmt.Sprintf("%v waiting for time to send vote", u.GetInfo()))
	//		first = false
	//	}
	//	txReceipt, err = u.Client.ContractCallEstimate("dna_estimateCallContract", payload)
	//	time.Sleep(requestRetryDelay)
	//}

	//if err == nil && !txReceipt.Success {
	//	err = errors.New(txReceipt.Error)
	//}
	//if err != nil {
	//	return errors.Wrap(err, "failed estimate method call")
	//}

	_, err := u.Client.ContractCall("dna_callContract", payload)
	//if err != nil {
	return errors.Wrap(err, "unable to send vote")
	//}

	//log.Info(fmt.Sprintf("%v sent vote, waiting for mined transaction %v...", u.GetInfo(), txHash))
	//if err := waitForMinedTransaction(u.Client, txHash, time.Minute*10); err != nil {
	//	return err
	//}
	//return nil
}

func waitForBlock(client *client.Client, height uint64, timeout time.Duration) error {
	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()
	timeoutReached := false
	go func() {
		<-timeoutTimer.C
		timeoutReached = true
	}()
	for {
		time.Sleep(requestRetryDelay)
		if timeoutReached {
			return errors.Errorf("timeout while waiting for block %v", height)
		}
		syncing, err := client.CheckSyncing()
		if err != nil {
			log.Warn(errors.Wrap(err, "unable to check syncing").Error())
			continue
		}
		if syncing.CurrentBlock < height {
			continue
		}
		return nil
	}
}

func waitForMinedTransaction(client *client.Client, txHash string, timeout time.Duration) error {
	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()
	timeoutReached := false
	go func() {
		<-timeoutTimer.C
		timeoutReached = true
	}()
	for {
		time.Sleep(requestRetryDelay)
		if timeoutReached {
			return errors.Errorf("tx %v is not mined", txHash)
		}
		tx, err := client.Transaction(txHash)
		if err != nil {
			log.Warn(errors.Wrapf(err, "unable to get transaction %v", txHash).Error())
			continue
		}
		if tx.BlockHash == (common.Hash{}) {
			continue
		}
		return nil
	}
}
