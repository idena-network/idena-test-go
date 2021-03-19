package process

import (
	"fmt"
	"github.com/google/tink/go/subtle/random"
	"github.com/idena-network/idena-go/api"
	"github.com/idena-network/idena-go/blockchain"
	"github.com/idena-network/idena-go/common"
	"github.com/idena-network/idena-go/common/eventbus"
	"github.com/idena-network/idena-go/common/hexutil"
	"github.com/idena-network/idena-go/crypto"
	"github.com/idena-network/idena-go/vm/embedded"
	"github.com/idena-network/idena-test-go/client"
	"github.com/idena-network/idena-test-go/log"
	"github.com/idena-network/idena-test-go/user"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"
	"strconv"
	"sync"
	"time"
)

const (
	ErrorMessageInvalidProof  = "invalid proof"
	ErrorMessageWrongVoteHash = "wrong vote hash"
	VotingTerminatedEventID   = eventbus.EventID("voting-terminated")
)

type VotingTerminatedEvent struct {
}

func (e *VotingTerminatedEvent) EventID() eventbus.EventID {
	return VotingTerminatedEventID
}

type contractsScenario1 struct {
	process *Process
	config  *contractsScenario1Config
	context *contractsScenario1Context
	logger  log.Logger
	bus     eventbus.Bus
}

type contractsScenario1Config struct {
	maxFee                decimal.Decimal
	votingDeploy          *contractsScenario1VotingDeployConfig
	votingCoins           *contractsScenario1VotingCoins
	refundableLocksDeploy *contractsScenario1RefundableLocksDeployConfig
	deposit               *contractsScenario1DepositConfig
	startVoting           *contractsScenario1StartVotingConfig
	voting                *contractsScenario1Voting
	finishVoting          *contractsScenario1FinishVotingConfig
	votingTermination     *contractsScenario1VotingTerminationConfig
}

type contractsScenario1RefundableLocksDeployConfig struct {
	sender     int
	answer0    *contractsScenario1RefundableLockDeployConfig
	answer1    *contractsScenario1RefundableLockDeployConfig
	answer2    *contractsScenario1RefundableLockDeployConfig
	answerNot0 *contractsScenario1RefundableLockDeployConfig
	answerNot1 *contractsScenario1RefundableLockDeployConfig
	answerNot2 *contractsScenario1RefundableLockDeployConfig
}

type contractsScenario1VotingDeployConfig struct {
	sender               int
	amount               decimal.Decimal
	startTimeSecOffset   *int
	fact                 []byte
	votingDuration       *int
	publicVotingDuration *int
	winnerThreshold      *int
	quorum               *int
	committeeSize        *int
	maxOptions           *int
	votingMinPayment     *decimal.Decimal
}

type contractsScenario1VotingCoins struct {
	sender int
	amount decimal.Decimal
}

type contractsScenario1StartVotingConfig struct {
	sender int
	amount decimal.Decimal
}

type contractsScenario1FinishVotingConfig struct {
	sender int
	amount decimal.Decimal
}

type contractsScenario1VotingTerminationConfig struct {
	sender int
	dest   *common.Address
}

type contractsScenario1DepositConfig struct {
	amount decimal.Decimal
}

type contractsScenario1RefundableLockDeployConfig struct {
	amount                   decimal.Decimal
	refundDelay              *int
	depositDeadlineSecOffset int
	fee                      int
}

type contractsScenario1Voting struct {
	vote byte
}

type contractsScenario1Context struct {
	appStartTime time.Time

	votingContractAddress common.Address
	startTime             time.Time
	votingDuration        uint64
	publicVotingDuration  uint64

	answer0ContractAddress    common.Address
	answer1ContractAddress    common.Address
	answer2ContractAddress    common.Address
	answerNot0ContractAddress common.Address
	answerNot1ContractAddress common.Address
	answerNot2ContractAddress common.Address

	startVotingTxHash string
	voteBlock         uint64
}

func intP(v int) *int {
	return &v
}

func decimalP(v decimal.Decimal) *decimal.Decimal {
	return &v
}

func addressP(v common.Address) *common.Address {
	return &v
}

func determineContractAddress(sender common.Address, epoch uint16, nonce uint32) common.Address {
	hash := crypto.Hash(append(append(sender.Bytes(), common.ToBytes(epoch)...), common.ToBytes(nonce)...))
	var result common.Address
	result.SetBytes(hash[:])
	return result
}

func getVotingMinPayment(u *user.User, contractAddress common.Address) (string, error) {
	var res string
	err := u.Client.ReadContractData(contractAddress, "votingMinPayment", "dna", &res)
	if err != nil {
		return "", errors.Wrap(err, "unable to get votingMinPayment")
	}
	return res, nil
}

func (process *Process) runContractsScenario1() {
	c := new(contractsScenario1)
	c.logger = log.New("component", "contracts-scenario-1")
	c.bus = eventbus.New()
	c.process = process
	c.config = &contractsScenario1Config{
		maxFee: decimal.NewFromInt(1000),
		votingDeploy: &contractsScenario1VotingDeployConfig{
			sender:               2,
			amount:               decimal.NewFromInt(100000),
			startTimeSecOffset:   intP(30),
			fact:                 []byte{0x1},
			votingDuration:       intP(5),
			publicVotingDuration: intP(5),
			winnerThreshold:      intP(50),
			quorum:               intP(50),
			committeeSize:        intP(3),
			maxOptions:           intP(3),
			votingMinPayment:     decimalP(decimal.NewFromInt(10)),
		},
		votingCoins: &contractsScenario1VotingCoins{
			sender: 0,
			amount: decimal.NewFromInt(3000),
		},
		refundableLocksDeploy: &contractsScenario1RefundableLocksDeployConfig{
			sender: 1,
			answer0: &contractsScenario1RefundableLockDeployConfig{
				amount:                   decimal.NewFromInt(100000),
				refundDelay:              intP(5),
				depositDeadlineSecOffset: 200,
				fee:                      50,
			},
			answer1: &contractsScenario1RefundableLockDeployConfig{
				amount:                   decimal.NewFromInt(100000),
				refundDelay:              intP(5),
				depositDeadlineSecOffset: 200,
				fee:                      50,
			},
			answer2: &contractsScenario1RefundableLockDeployConfig{
				amount:                   decimal.NewFromInt(100000),
				refundDelay:              intP(5),
				depositDeadlineSecOffset: 200,
				fee:                      50,
			},
			answerNot0: &contractsScenario1RefundableLockDeployConfig{
				amount:                   decimal.NewFromInt(100000),
				refundDelay:              intP(5),
				depositDeadlineSecOffset: 999999,
				fee:                      50,
			},
			answerNot1: &contractsScenario1RefundableLockDeployConfig{
				amount:                   decimal.NewFromInt(100000),
				refundDelay:              intP(5),
				depositDeadlineSecOffset: 999999,
				fee:                      50,
			},
			answerNot2: &contractsScenario1RefundableLockDeployConfig{
				amount:                   decimal.NewFromInt(100000),
				refundDelay:              intP(5),
				depositDeadlineSecOffset: 999999,
				fee:                      50,
			},
		},
		deposit: &contractsScenario1DepositConfig{
			amount: decimal.NewFromInt(1000),
		},
		startVoting: &contractsScenario1StartVotingConfig{
			sender: 2,
			amount: decimal.NewFromInt(10),
		},
		voting: &contractsScenario1Voting{
			vote: 2,
		},
		finishVoting: &contractsScenario1FinishVotingConfig{
			sender: 0,
			amount: decimal.NewFromInt(2000),
		},
		votingTermination: &contractsScenario1VotingTerminationConfig{
			sender: 2,
			dest:   addressP(common.Address{}),
		},
	}
	c.context = new(contractsScenario1Context)
	c.context.appStartTime = time.Now()

	if err := c.initContracts(); err != nil {
		process.handleError(err, "")
	}

	okPushChan := make(chan struct{})
	go c.loopLockPushingAndRefunding(okPushChan)

	if err := c.sendDeposits(); err != nil {
		process.handleError(err, "")
	}

	timeToWait := c.context.startTime.Add(time.Second * 30)
	c.logger.Info(fmt.Sprintf("Start waiting for time %v to start voting...", timeToWait))
	<-time.After(timeToWait.Sub(time.Now()))

	if err := c.performVoting(); err != nil {
		process.handleError(err, "")
	}

	if err := c.finishVoting(); err != nil {
		process.handleError(err, "")
	}

	if err := c.terminateContract(); err != nil {
		process.handleError(err, "")
	}

	c.bus.Publish(&VotingTerminatedEvent{})

	<-okPushChan
	c.logger.Info("Test completed")
}

func (c *contractsScenario1) initContracts() error {
	if err := c.initVotingContract(); err != nil {
		return err
	}

	if err := c.initRefundableLockContracts(); err != nil {
		return err
	}

	return nil
}

func (c *contractsScenario1) initVotingContract() error {
	if err := c.deployContract(); err != nil {
		return err
	}

	if err := c.sendCoinsToContract(); err != nil {
		return err
	}

	return nil
}

func (c *contractsScenario1) deployContract() error {
	u := c.process.users[c.config.votingDeploy.sender]

	c.logger.Info(fmt.Sprintf("%v waiting for online...", u.GetInfo()))
	if err := c.process.waitForOnline(u, time.Minute); err != nil {
		return errors.Wrapf(err, "%v node is not online", u.GetInfo())
	}

	codeHash := embedded.OracleVotingContract

	u.Client.Lock()
	defer u.Client.Unlock()

	epoch, nonce := c.process.getLatestTxNonce(u)
	if epoch < c.process.getEpoch(u).Epoch {
		epoch = c.process.getEpoch(u).Epoch
		nonce = 0
	}
	//epoch := c.process.getEpoch(u).Epoch
	//nonce := c.process.getBalance(u).Nonce

	expectedContractAddress := determineContractAddress(common.HexToAddress(u.Address), epoch, nonce+1)
	c.context.votingContractAddress = expectedContractAddress

	c.logger.Info(fmt.Sprintf("%v expected contract address: %v", u.GetInfo(), expectedContractAddress.Hex()))

	startTime := c.context.appStartTime.Add(time.Second * time.Duration(*c.config.votingDeploy.startTimeSecOffset))
	c.context.startTime = startTime
	args := api.DynamicArgs{
		&api.DynamicArg{
			Index:  0,
			Format: "hex",
			Value:  hexutil.Encode(c.config.votingDeploy.fact),
		},
		&api.DynamicArg{
			Index:  1,
			Format: "uint64",
			Value:  strconv.FormatUint(uint64(startTime.Unix()), 10),
		},
	}
	if c.config.votingDeploy.votingDuration != nil {
		c.context.votingDuration = uint64(*c.config.votingDeploy.votingDuration)
		args = append(args, &api.DynamicArg{
			Index:  2,
			Format: "uint64",
			Value:  strconv.FormatInt(int64(*c.config.votingDeploy.votingDuration), 10),
		})
	}
	if c.config.votingDeploy.publicVotingDuration != nil {
		c.context.publicVotingDuration = uint64(*c.config.votingDeploy.publicVotingDuration)
		args = append(args, &api.DynamicArg{
			Index:  3,
			Format: "uint64",
			Value:  strconv.FormatInt(int64(*c.config.votingDeploy.publicVotingDuration), 10),
		})
	}
	if c.config.votingDeploy.winnerThreshold != nil {
		args = append(args, &api.DynamicArg{
			Index:  4,
			Format: "uint64",
			Value:  strconv.FormatInt(int64(*c.config.votingDeploy.winnerThreshold), 10),
		})
	}
	if c.config.votingDeploy.quorum != nil {
		args = append(args, &api.DynamicArg{
			Index:  5,
			Format: "uint64",
			Value:  strconv.FormatInt(int64(*c.config.votingDeploy.quorum), 10),
		})
	}
	if c.config.votingDeploy.committeeSize != nil {
		args = append(args, &api.DynamicArg{
			Index:  6,
			Format: "uint64",
			Value:  strconv.FormatInt(int64(*c.config.votingDeploy.committeeSize), 10),
		})
	}
	if c.config.votingDeploy.maxOptions != nil {
		args = append(args, &api.DynamicArg{
			Index:  7,
			Format: "uint64",
			Value:  strconv.FormatInt(int64(*c.config.votingDeploy.maxOptions), 10),
		})
	}
	if c.config.votingDeploy.votingMinPayment != nil {
		votingMinPayment := blockchain.ConvertToInt(*c.config.votingDeploy.votingMinPayment).String()
		args = append(args, &api.DynamicArg{
			Index:  8,
			Format: "bigint",
			Value:  votingMinPayment,
		})
	}

	params := api.DeployArgs{
		From:     common.HexToAddress(u.Address),
		CodeHash: codeHash.Bytes(),
		Amount:   c.config.votingDeploy.amount,
		Args:     args,
		MaxFee:   c.config.maxFee,
	}

	payload := []api.DeployArgs{params}
	return deployContract(u, payload, true, c.logger)
}

func (c *contractsScenario1) sendCoinsToContract() error {
	if c.config.votingCoins == nil {
		c.logger.Info("No configured sender for sending coins to contract")
		return nil
	}
	u := c.process.users[c.config.votingCoins.sender]
	to := c.context.votingContractAddress.Hex()
	amount, _ := c.config.votingCoins.amount.Float64()
	maxFee, _ := c.config.maxFee.Float64()
	txHash, err := u.Client.SendTransaction(0, u.Address, &to, float32(amount), float32(maxFee), nil)
	if err != nil {
		return errors.Wrapf(err, "%v unable to send coins to contract", u.GetInfo())
	}

	c.logger.Info(fmt.Sprintf("%v sent coins to contract address %v, txHash %v. Waiting for tx is mined...", u.GetInfo(), to, txHash))

	err = waitForMinedTransaction(u.Client, txHash, time.Minute)
	if err != nil {
		return errors.Wrapf(err, "tx %v is not mined", txHash)
	}
	c.logger.Info(fmt.Sprintf("%v mined transaction %v", u.GetInfo(), txHash))
	return nil
}

func (c *contractsScenario1) initRefundableLockContracts() error {
	u := c.process.users[c.config.refundableLocksDeploy.sender]

	c.logger.Info(fmt.Sprintf("%v waiting for online...", u.GetInfo()))
	if err := c.process.waitForOnline(u, time.Minute); err != nil {
		return errors.Wrapf(err, "%v node is not online", u.GetInfo())
	}

	u.Client.Lock()
	defer u.Client.Unlock()

	epoch, nonce := c.process.getLatestTxNonce(u)
	if epoch < c.process.getEpoch(u).Epoch {
		epoch = c.process.getEpoch(u).Epoch
		nonce = 0
	}
	//epoch := c.process.getEpoch(u).Epoch
	//nonce := c.process.getBalance(u).Nonce

	expectedAnswer0ContractAddress := determineContractAddress(common.HexToAddress(u.Address), epoch, nonce+1)
	expectedAnswer1ContractAddress := determineContractAddress(common.HexToAddress(u.Address), epoch, nonce+2)
	expectedAnswer2ContractAddress := determineContractAddress(common.HexToAddress(u.Address), epoch, nonce+3)
	expectedAnswerNot0ContractAddress := determineContractAddress(common.HexToAddress(u.Address), epoch, nonce+4)
	expectedAnswerNot1ContractAddress := determineContractAddress(common.HexToAddress(u.Address), epoch, nonce+5)
	expectedAnswerNot2ContractAddress := determineContractAddress(common.HexToAddress(u.Address), epoch, nonce+6)

	c.context.answer0ContractAddress = expectedAnswer0ContractAddress
	c.context.answer1ContractAddress = expectedAnswer1ContractAddress
	c.context.answer2ContractAddress = expectedAnswer2ContractAddress
	c.context.answerNot0ContractAddress = expectedAnswerNot0ContractAddress
	c.context.answerNot1ContractAddress = expectedAnswerNot1ContractAddress
	c.context.answerNot2ContractAddress = expectedAnswerNot2ContractAddress

	c.logger.Info(fmt.Sprintf("expected refundable lock contracts addresses: %v, %v, %v, %v, %v, %v",
		expectedAnswer0ContractAddress.Hex(),
		expectedAnswer1ContractAddress.Hex(),
		expectedAnswer2ContractAddress.Hex(),
		expectedAnswerNot0ContractAddress.Hex(),
		expectedAnswerNot1ContractAddress.Hex(),
		expectedAnswerNot2ContractAddress.Hex(),
	))

	cfg := c.config.refundableLocksDeploy.answer0
	depositDeadline := c.context.appStartTime.Add(time.Second * time.Duration(cfg.depositDeadlineSecOffset))
	if err := c.deployRefundableLockContract(u, false, cfg.amount,
		c.config.maxFee, 0, nil, addressP(expectedAnswerNot0ContractAddress),
		cfg.refundDelay, depositDeadline.Unix(), cfg.fee); err != nil {
		return err
	}

	cfg = c.config.refundableLocksDeploy.answer1
	depositDeadline = c.context.appStartTime.Add(time.Second * time.Duration(cfg.depositDeadlineSecOffset))
	if err := c.deployRefundableLockContract(u, false, cfg.amount,
		c.config.maxFee, 1, nil, addressP(expectedAnswerNot1ContractAddress),
		cfg.refundDelay, depositDeadline.Unix(), cfg.fee); err != nil {
		return err
	}

	cfg = c.config.refundableLocksDeploy.answer2
	depositDeadline = c.context.appStartTime.Add(time.Second * time.Duration(cfg.depositDeadlineSecOffset))
	if err := c.deployRefundableLockContract(u, false, cfg.amount,
		c.config.maxFee, 2, nil, addressP(expectedAnswerNot2ContractAddress),
		cfg.refundDelay, depositDeadline.Unix(), cfg.fee); err != nil {
		return err
	}

	cfg = c.config.refundableLocksDeploy.answerNot0
	depositDeadline = c.context.appStartTime.Add(time.Second * time.Duration(cfg.depositDeadlineSecOffset))
	if err := c.deployRefundableLockContract(u, false, cfg.amount,
		c.config.maxFee, 1, addressP(expectedAnswer1ContractAddress), addressP(expectedAnswer2ContractAddress),
		cfg.refundDelay, depositDeadline.Unix(), cfg.fee); err != nil {
		return err
	}

	cfg = c.config.refundableLocksDeploy.answerNot1
	depositDeadline = c.context.appStartTime.Add(time.Second * time.Duration(cfg.depositDeadlineSecOffset))
	if err := c.deployRefundableLockContract(u, false, cfg.amount,
		c.config.maxFee, 0, addressP(expectedAnswer0ContractAddress), addressP(expectedAnswer2ContractAddress),
		cfg.refundDelay, depositDeadline.Unix(), cfg.fee); err != nil {
		return err
	}

	cfg = c.config.refundableLocksDeploy.answerNot2
	depositDeadline = c.context.appStartTime.Add(time.Second * time.Duration(cfg.depositDeadlineSecOffset))
	if err := c.deployRefundableLockContract(u, true, cfg.amount,
		c.config.maxFee, 0, addressP(expectedAnswer0ContractAddress), addressP(expectedAnswer1ContractAddress),
		cfg.refundDelay, depositDeadline.Unix(), cfg.fee); err != nil {
		return err
	}

	return nil
}

func (c *contractsScenario1) deployRefundableLockContract(
	u *user.User, waitForMinedTx bool,
	amount,
	maxFee decimal.Decimal,
	successValue int,
	successAddress, failAddress *common.Address,
	refundDelay *int,
	depositDeadline int64,
	fee int,
) error {
	codeHash := embedded.RefundableOracleLockContract

	var args api.DynamicArgs

	args = append(args, &api.DynamicArg{
		Index:  0,
		Format: "hex",
		Value:  c.context.votingContractAddress.Hex(),
	})

	args = append(args, &api.DynamicArg{
		Index:  1,
		Format: "byte",
		Value:  strconv.Itoa(successValue),
	})

	if successAddress != nil {
		args = append(args, &api.DynamicArg{
			Index:  2,
			Format: "hex",
			Value:  successAddress.Hex(),
		})
	}

	if failAddress != nil {
		args = append(args, &api.DynamicArg{
			Index:  3,
			Format: "hex",
			Value:  failAddress.Hex(),
		})
	}

	if refundDelay != nil {
		args = append(args, &api.DynamicArg{
			Index:  4,
			Format: "uint64",
			Value:  strconv.FormatInt(int64(*refundDelay), 10),
		})
	}

	args = append(args, &api.DynamicArg{
		Index:  5,
		Format: "uint64",
		Value:  strconv.FormatInt(depositDeadline, 10),
	})

	args = append(args, &api.DynamicArg{
		Index:  6,
		Format: "byte",
		Value:  strconv.Itoa(fee),
	})

	params := api.DeployArgs{
		From:     common.HexToAddress(u.Address),
		CodeHash: codeHash.Bytes(),
		Amount:   amount,
		Args:     args,
		MaxFee:   maxFee,
	}

	payload := []api.DeployArgs{params}
	return deployContract(u, payload, waitForMinedTx, c.logger)
}

func deployContract(u *user.User, payload interface{}, waitForMinedTx bool, logger log.Logger) error {
	txReceipt, err := u.Client.ContractCallEstimate("contract_estimateDeploy", payload)
	if err == nil && !txReceipt.Success {
		err = errors.New(txReceipt.Error)
	}
	if err != nil {
		return errors.Wrapf(err, "%v unable to call contract_estimateDeploy", u.GetInfo())
	}
	logger.Info(fmt.Sprintf("%v contract_estimateDeploy ok, contractAddress: %v, gasUsed %v, gasCost %v, txFee %v", u.GetInfo(),
		txReceipt.Contract.Hex(),
		txReceipt.GasUsed,
		txReceipt.GasCost,
		txReceipt.TxFee,
	))

	txHash, err := u.Client.ContractCallWithNoLock("contract_deploy", payload)
	if err != nil {
		return errors.Wrapf(err, "%v unable to call contract_deploy", u.GetInfo())
	}

	if !waitForMinedTx {
		logger.Info(fmt.Sprintf("%v contract_deploy ok, txHash %v", u.GetInfo(), txHash))
		return nil
	}

	logger.Info(fmt.Sprintf("%v contract_deploy ok, txHash %v. Waiting for tx is mined...", u.GetInfo(), txHash))

	err = waitForMinedTransaction(u.Client, txHash, time.Minute)
	if err != nil {
		return errors.Wrapf(err, "tx %v is not mined", txHash)
	}

	logger.Info(fmt.Sprintf("%v mined transaction %v", u.GetInfo(), txHash))
	return nil
}

func (c *contractsScenario1) sendDeposits() error {
	wg := sync.WaitGroup{}
	wg.Add(len(c.process.users))
	okChan := make(chan struct{})
	errorChan := make(chan error)

	go func() {
		wg.Wait()
		okChan <- struct{}{}
	}()

	for _, u := range c.process.users {
		go func(u *user.User) {
			var to common.Address
			switch u.Index % 3 {
			case 0:
				to = c.context.answer0ContractAddress
			case 1:
				to = c.context.answer1ContractAddress
			case 2:
				to = c.context.answer2ContractAddress
			}
			c.logger.Info(fmt.Sprintf("%v sending deposit %v to %v", u.GetInfo(), c.config.deposit.amount.String(), to.Hex()))
			if err := c.sendDeposit(u, to, c.config.deposit.amount); err != nil {
				err = errors.Wrapf(err, "%v unable to send deposit %v to %v", u.GetInfo(), c.config.deposit.amount.String(), to.Hex())
				errorChan <- err
			} else {
				c.logger.Info(fmt.Sprintf("%v sent deposit %v to %v", u.GetInfo(), c.config.deposit.amount.String(), to.Hex()))
				wg.Done()
			}
		}(u)
	}

	select {
	case err := <-errorChan:
		return err
	case <-okChan:
		return nil
	}
}

func (c *contractsScenario1) sendDeposit(u *user.User, contractAddress common.Address, amount decimal.Decimal) error {
	payload := []api.CallArgs{
		{
			Contract: contractAddress,
			Amount:   amount,
			Method:   "deposit",
			MaxFee:   c.config.maxFee,
		},
	}
	txReceipt, err := u.Client.ContractCallEstimate("contract_estimateCall", payload)
	if err == nil && !txReceipt.Success {
		err = errors.New(txReceipt.Error)
	}
	if err != nil {
		return errors.Wrap(err, "failed estimate method call")
	}

	txHash, err := u.Client.ContractCall("contract_call", payload)
	if err != nil {
		return errors.Wrap(err, "unable to send vote proof")
	}

	c.logger.Info(fmt.Sprintf("%v sent contract_call deposit to contract %v, txHash %v. Waiting for tx is mined...", u.GetInfo(), contractAddress.Hex(), txHash))

	if err := waitForMinedTransaction(u.Client, txHash, time.Minute); err != nil {
		return errors.Wrapf(err, "tx %v is not mined", txHash)
	}
	return nil
}

func (c *contractsScenario1) performVoting() error {
	if err := c.startVoting(); err != nil {
		return err
	}

	wg := sync.WaitGroup{}
	wg.Add(len(c.process.users))
	errChan := make(chan error)
	okChan := make(chan struct{})
	for _, u := range c.process.users {
		go func(u *user.User) {
			err := c.performUserVoting(u)
			if err != nil {
				errChan <- err
			} else {
				wg.Done()
			}
		}(u)
	}
	go func() {
		wg.Wait()
		okChan <- struct{}{}
	}()

	select {
	case err := <-errChan:
		return err
	case <-okChan:
	}
	return nil
}

func (c *contractsScenario1) startVoting() error {

	u := c.process.users[c.config.startVoting.sender]

	args := api.DynamicArgs{}
	params := api.CallArgs{
		From:     common.HexToAddress(u.Address),
		Contract: c.context.votingContractAddress,
		Method:   "startVoting",
		Amount:   c.config.startVoting.amount,
		Args:     args,
		MaxFee:   c.config.maxFee,
	}
	payload := []api.CallArgs{params}

	txReceipt, err := u.Client.ContractCallEstimate("contract_estimateCall", payload)
	if err == nil && !txReceipt.Success {
		err = errors.New(txReceipt.Error)
	}
	if err != nil {
		return errors.Wrapf(err, "%v unable to call contract_estimateCall startVoting", u.GetInfo())
	}
	c.logger.Info(fmt.Sprintf("%v contract_estimateCall startVoting ok, contractAddress: %v, gasUsed %v, gasCost %v, txFee %v", u.GetInfo(),
		txReceipt.Contract.Hex(),
		txReceipt.GasUsed,
		txReceipt.GasCost,
		txReceipt.TxFee,
	))

	txHash, err := u.Client.ContractCall("contract_call", payload)
	if err != nil {
		return errors.Wrapf(err, "%v unable to call contract_call startVoting", u.GetInfo())
	}
	c.context.startVotingTxHash = txHash

	c.logger.Info(fmt.Sprintf("%v sent contract_call startVoting to contract %v, txHash %v. Waiting for tx is mined...", u.GetInfo(), c.context.votingContractAddress.Hex(), txHash))

	err = waitForMinedTransaction(u.Client, txHash, time.Minute)
	if err != nil {
		return errors.Wrapf(err, "tx %v is not mined", txHash)
	}

	c.logger.Info(fmt.Sprintf("%v mined transaction %v", u.GetInfo(), txHash))
	return nil
}

func (c *contractsScenario1) performUserVoting(u *user.User) error {

	if c.config.voting == nil {
		c.logger.Info(fmt.Sprintf("%v voting is not configured", u.GetInfo()))
		return nil
	}

	txHash := c.context.startVotingTxHash
	c.logger.Info(fmt.Sprintf("%v waiting for mined transaction %v", u.GetInfo(), txHash))

	if err := waitForMinedTransaction(u.Client, txHash, time.Minute); err != nil {
		return errors.Wrapf(err, "tx %v is not mined", txHash)
	}
	c.logger.Info(fmt.Sprintf("%v mined transaction %v", u.GetInfo(), txHash))

	// Get proof
	proof, err := getProof(u, c.context.votingContractAddress)
	if err != nil {
		return errors.Wrapf(err, "%v unable to get proof", u.GetInfo())
	}
	inCommittee := len(proof) > 0
	if inCommittee {
		c.logger.Info(fmt.Sprintf("%v user is in committee, proof: %v", u.GetInfo(), proof))
	} else {
		c.logger.Info(fmt.Sprintf("%v user is not in committee", u.GetInfo()))
	}

	// Get vote hash
	vote := c.config.voting.vote
	salt := random.GetRandomBytes(20)
	voteHash, err := getVoteHash(u, vote, salt, c.context.votingContractAddress)
	if err != nil {
		return errors.Wrapf(err, "%v unable to get vote hash", u.GetInfo())
	}
	c.logger.Info(fmt.Sprintf("%v voteHash: %v", u.GetInfo(), voteHash))

	// Get voting min payment
	votingMinPayment, err := getVotingMinPayment(u, c.context.votingContractAddress)
	if err != nil {
		return errors.Wrapf(err, "%v unable to get voting min payment", u.GetInfo())
	}
	c.logger.Info(fmt.Sprintf("%v votingMinPayment: %v", u.GetInfo(), votingMinPayment))

	// Get vote block
	voteBlock, err := getVoteBlock(u, c.context.votingContractAddress)
	if err != nil {
		return errors.Wrapf(err, "%v unable to get vote block", u.GetInfo())
	}
	c.context.voteBlock = voteBlock
	c.logger.Info(fmt.Sprintf("%v voteBlock: %v", u.GetInfo(), voteBlock))

	// Send vote proof
	if err := sendVoteProof(u, c.context.votingContractAddress, votingMinPayment, voteHash); err != nil {
		if errors.Cause(err).Error() == ErrorMessageInvalidProof && !inCommittee {
			c.logger.Info(fmt.Sprintf("%v can't send vote proof as it is not in committee", u.GetInfo()))
		} else {
			return errors.Wrapf(err, "%v unable to send vote proof", u.GetInfo())
		}
	} else {
		c.logger.Info(fmt.Sprintf("%v sent vote proof", u.GetInfo()))
	}

	// Send vote
	if inCommittee {
		if err := sendVote(u, c.context.votingContractAddress, voteBlock, vote, salt); err != nil {
			if errors.Cause(err).Error() == ErrorMessageWrongVoteHash && !inCommittee {
				c.logger.Info(fmt.Sprintf("%v can't send vote as it is not in committee", u.GetInfo()))
			} else {
				return errors.Wrapf(err, "%v unable to send vote", u.GetInfo())
			}
		} else {
			c.logger.Info(fmt.Sprintf("%v sent vote", u.GetInfo()))
		}
	}
	return nil
}

func getVoteHash(u *user.User, vote byte, salt hexutil.Bytes, contractAddress common.Address) (string, error) {
	var res string
	err := u.Client.ReadonlyCallContract([]api.ReadonlyCallArgs{
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

func sendVote(u *user.User, contractAddress common.Address, voteBlock uint64, vote byte, salt hexutil.Bytes) error {
	payload := []api.CallArgs{
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
	//txReceipt, err := u.Client.ContractCallEstimate("contract_estimateCall", payload)
	// Wait for time to send vote
	//first := true
	//for err == nil && !txReceipt.Success && txReceipt.Error == ErrorMessageWrongBlockNumber {
	//	if first {
	//		c.logger.Info(fmt.Sprintf("%v waiting for time to send vote", u.GetInfo()))
	//		first = false
	//	}
	//	txReceipt, err = u.Client.ContractCallEstimate("contract_estimateCall", payload)
	//	time.Sleep(requestRetryDelay)
	//}
	//if err == nil && !txReceipt.Success {
	//	err = errors.New(txReceipt.Error)
	//}
	//if err != nil {
	//	return errors.Wrap(err, "failed estimate method call")
	//}
	_, err := u.Client.ContractCall("contract_call", payload)
	//if err != nil {
	return errors.Wrap(err, "unable to send vote")
	//}
	//c.logger.Info(fmt.Sprintf("%v sent vote, waiting for mined transaction %v...", u.GetInfo(), txHash))
	//if err := waitForMinedTransaction(u.Client, txHash, time.Minute*10); err != nil {
	//	return err
	//}
	//return nil
}

func getProof(u *user.User, contractAddress common.Address) (string, error) {
	var res string
	err := u.Client.ReadonlyCallContract([]api.ReadonlyCallArgs{
		{
			Method:   "proof",
			Contract: contractAddress,
			Format:   "hex",
			Args: api.DynamicArgs{
				{
					Index:  0,
					Format: "hex",
					Value:  u.Address,
				},
			},
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

func getVoteBlock(u *user.User, contractAddress common.Address) (uint64, error) {
	var res uint64
	err := u.Client.ReadonlyCallContract([]api.ReadonlyCallArgs{
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

func sendVoteProof(u *user.User, contractAddress common.Address, votingMinPayment, voteHash /*, proof*/ string) error {
	payload := []api.CallArgs{
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
				/*{
					Index:  1,
					Format: "hex",
					Value:  proof,
				},*/
			},
		},
	}
	txReceipt, err := u.Client.ContractCallEstimate("contract_estimateCall", payload)
	if err == nil && !txReceipt.Success {
		err = errors.New(txReceipt.Error)
	}
	if err != nil {
		return errors.Wrap(err, "failed estimate method call")
	}
	txHash, err := u.Client.ContractCall("contract_call", payload)
	if err != nil {
		return errors.Wrap(err, "unable to send vote proof")
	}
	if err := waitForMinedTransaction(u.Client, txHash, time.Minute); err != nil {
		return errors.Wrapf(err, "tx %v is not mined", txHash)
	}
	return nil
}

func (c *contractsScenario1) finishVoting() error {
	u := c.process.users[c.config.finishVoting.sender]
	syncing, err := u.Client.CheckSyncing()
	if err != nil {
		return errors.Wrapf(err, "%v unable to check sync", u.GetInfo())
	}
	highestBlock := syncing.HighestBlock
	blockToWait := c.context.voteBlock + c.context.publicVotingDuration
	timeout := time.Duration(blockToWait-highestBlock)*time.Second*20 + time.Minute
	c.logger.Info(fmt.Sprintf("%v start waiting for block %v to finish voting...", u.GetInfo(), blockToWait))
	if err := waitForBlock(u.Client, blockToWait, timeout); err != nil {
		return err
	}

	if err := finishVoting(u, c.context.votingContractAddress, c.config.finishVoting.amount, c.config.maxFee); err != nil {
		return errors.Wrapf(err, "%v unable to finish voting", u.GetInfo())
	}
	c.logger.Info(fmt.Sprintf("%v finished voting", u.GetInfo()))
	return nil
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
			return errors.Wrap(err, "unable to check syncing")
		}
		if syncing.CurrentBlock < height {
			continue
		}
		return nil
	}
}

func finishVoting(u *user.User, contractAddress common.Address, amount decimal.Decimal, maxFee decimal.Decimal) error {
	payload := []api.CallArgs{
		{
			Contract: contractAddress,
			Method:   "finishVoting",
			Amount:   amount,
			MaxFee:   maxFee,
		},
	}
	txReceipt, err := u.Client.ContractCallEstimate("contract_estimateCall", payload)
	if err == nil && !txReceipt.Success {
		err = errors.New(txReceipt.Error)
	}
	if err != nil {
		return errors.Wrap(err, "failed estimate method call")
	}
	txHash, err := u.Client.ContractCall("contract_call", payload)
	if err != nil {
		return errors.Wrap(err, "failed method call")
	}
	if err := waitForMinedTransaction(u.Client, txHash, time.Minute); err != nil {
		return errors.Wrapf(err, "tx %v is not mined", txHash)
	}
	return nil
}

func (c *contractsScenario1) terminateContract() error {
	u := c.process.users[c.config.votingTermination.sender]

	syncing, err := u.Client.CheckSyncing()
	if err != nil {
		return errors.Wrapf(err, "%v unable to check sync", u.GetInfo())
	}
	highestBlock := syncing.HighestBlock
	blockToWait := c.context.voteBlock + c.context.votingDuration + c.context.publicVotingDuration + 8
	timeout := time.Duration(blockToWait-highestBlock)*time.Second*20 + time.Minute
	c.logger.Info(fmt.Sprintf("%v start waiting for block %v to terminate contract...", u.GetInfo(), blockToWait))
	if err := waitForBlock(u.Client, blockToWait, timeout); err != nil {
		return err
	}

	var args api.DynamicArgs
	if c.config.votingTermination.dest != nil {
		args = append(args, &api.DynamicArg{
			Index:  0,
			Format: "hex",
			Value:  c.config.votingTermination.dest.Hex(),
		})
	}

	params := api.TerminateArgs{
		From:     common.HexToAddress(u.Address),
		Contract: c.context.votingContractAddress,
		Args:     args,
		MaxFee:   c.config.maxFee,
	}

	payload := []api.TerminateArgs{params}

	txReceipt, err := u.Client.ContractCallEstimate("contract_estimateTerminate", payload)
	if err == nil && !txReceipt.Success {
		err = errors.New(txReceipt.Error)
	}
	if err != nil {
		return errors.Wrapf(err, "%v unable to call contract_estimateTerminate", u.GetInfo())
	}
	c.logger.Info(fmt.Sprintf("%v contract_estimateTerminate ok, contractAddress: %v, gasUsed %v, gasCost %v, txFee %v", u.GetInfo(),
		txReceipt.Contract.Hex(),
		txReceipt.GasUsed,
		txReceipt.GasCost,
		txReceipt.TxFee,
	))

	txHash, err := u.Client.ContractCall("contract_terminate", payload)
	if err != nil {
		return errors.Wrapf(err, "%v unable to call contract_terminate", u.GetInfo())
	}

	c.logger.Info(fmt.Sprintf("%v contract_terminate ok, txHash %v. Waiting for tx is mined...", u.GetInfo(), txHash))

	err = waitForMinedTransaction(u.Client, txHash, time.Minute)
	if err != nil {
		return errors.Wrapf(err, "tx %v is not mined", txHash)
	}

	c.logger.Info(fmt.Sprintf("%v mined transaction %v", u.GetInfo(), txHash))

	return nil
}

func (c *contractsScenario1) loopLockPushingAndRefunding(resChan chan struct{}) {

	u := c.process.users[1] // TODO configure index

	wg := sync.WaitGroup{}
	wg.Add(7)
	errChan := make(chan error)
	okChan := make(chan struct{})

	mutex := &sync.Mutex{}

	push := func(contractAddress common.Address) {
		if err := loopLockPushing(u, contractAddress, c.config.maxFee, mutex, c.logger, c.bus); err != nil {
			errChan <- err
		} else {
			wg.Done()
		}
	}

	refund := func(contractAddress common.Address) {
		if err := loopLockRefunding(u, contractAddress, c.config.maxFee, mutex, c.logger); err != nil {
			errChan <- err
		} else {
			wg.Done()
		}
	}

	go func() {
		wg.Wait()
		okChan <- struct{}{}
	}()

	go push(c.context.answer0ContractAddress)
	go push(c.context.answer1ContractAddress)
	go push(c.context.answer2ContractAddress)
	go push(c.context.answerNot0ContractAddress)
	go push(c.context.answerNot1ContractAddress)
	go push(c.context.answerNot2ContractAddress)

	switch c.config.voting.vote {
	case 0:
		go refund(c.context.answer0ContractAddress)
	case 1:
		go refund(c.context.answer1ContractAddress)
	case 2:
		go refund(c.context.answer2ContractAddress)
	}

	select {
	case err := <-errChan:
		c.process.handleError(err, "")
	case <-okChan:
		resChan <- struct{}{}
	}
}

func loopLockPushing(u *user.User, contractAddress common.Address, maxFee decimal.Decimal, mutex *sync.Mutex, logger log.Logger, bus eventbus.Bus) error {
	votingTerminated := false
	bus.Subscribe(VotingTerminatedEventID, func(e eventbus.Event) {
		votingTerminated = true
	})
	const errVoteIsNotCompleted = "failed estimate method call: voting is not completed"
	logger.Info(fmt.Sprintf("%v trying to send push to contract %v...", u.GetInfo(), contractAddress.Hex()))
	for {
		err := sendPush(u, contractAddress, decimal.NewFromInt(0), maxFee, mutex)
		if err == nil {
			logger.Info(fmt.Sprintf("%v sent push to contract %v", u.GetInfo(), contractAddress.Hex()))
			if votingTerminated {
				logger.Info(fmt.Sprintf("%v contract %v is terminated", u.GetInfo(), contractAddress.Hex()))
				return nil
			}
			time.Sleep(requestRetryDelay)
			continue
		}
		if err.Error() != errVoteIsNotCompleted {
			return errors.Wrapf(err, "%v unable to send push to contract %v", u.GetInfo(), contractAddress.Hex())
		}
		time.Sleep(requestRetryDelay)
		continue
	}
}

func loopLockRefunding(u *user.User, contractAddress common.Address, maxFee decimal.Decimal, mutex *sync.Mutex, logger log.Logger) error {
	const errNotUnlockedRefund = "failed estimate method call: state is not unlocked_refund"
	const errNotRefundBlock = "failed estimate method call: block height is less than refundBlock"
	logger.Info(fmt.Sprintf("%v trying to send refund to contract %v...", u.GetInfo(), contractAddress.Hex()))
	for {
		err := sendRefund(u, contractAddress, decimal.NewFromInt(0), maxFee, mutex)
		if err != nil {
			if err.Error() != errNotUnlockedRefund && err.Error() != errNotRefundBlock {
				return errors.Wrapf(err, "%v unable to send refund to contract %v", u.GetInfo(), contractAddress.Hex())
			}
			time.Sleep(requestRetryDelay)

			continue
		}
		logger.Info(fmt.Sprintf("%v sent refund to contract %v", u.GetInfo(), contractAddress.Hex()))
		return nil
	}
}

func sendPush(u *user.User, contractAddress common.Address, amount decimal.Decimal, maxFee decimal.Decimal, mutex *sync.Mutex) error {
	return callWithLock(u, contractAddress, "push", amount, maxFee, mutex)
}

func sendRefund(u *user.User, contractAddress common.Address, amount decimal.Decimal, maxFee decimal.Decimal, mutex *sync.Mutex) error {
	return callWithLock(u, contractAddress, "refund", amount, maxFee, mutex)
}

func callWithLock(u *user.User, contractAddress common.Address, method string, amount decimal.Decimal, maxFee decimal.Decimal, mutex *sync.Mutex) error {
	payload := []api.CallArgs{
		{
			Contract: contractAddress,
			Amount:   amount,
			Method:   method,
			MaxFee:   maxFee,
		},
	}
	txReceipt, err := u.Client.ContractCallEstimate("contract_estimateCall", payload)
	if err == nil && !txReceipt.Success {
		err = errors.New(txReceipt.Error)
	}
	if err != nil {
		return errors.Wrap(err, "failed estimate method call")
	}

	mutex.Lock()
	txHash, err := u.Client.ContractCall("contract_call", payload)
	mutex.Unlock()
	if err != nil {
		return errors.Wrap(err, "unable to send vote proof")
	}
	if err := waitForMinedTransaction(u.Client, txHash, time.Minute); err != nil {
		return errors.Wrapf(err, "tx %v is not mined", txHash)
	}
	return nil
}
