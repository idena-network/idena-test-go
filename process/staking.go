package process

import (
	"fmt"
	"github.com/idena-network/idena-test-go/log"
	"github.com/idena-network/idena-test-go/user"
	"time"
)

func (process *Process) addStake(u user.User) {
	epoch := process.getCurrentTestIndex()
	epochAddStakes, ok := process.sc.AddStakes[epoch]
	if !ok {
		return
	}
	amount, ok := epochAddStakes[u.GetIndex()]
	if !ok {
		return
	}
	process.sendAddStakeTx(u, amount)
}

func (process *Process) sendAddStakeTx(u user.User, amount float32) {
	const AddStakeTx = 0x16
	address := u.GetAddress()
	hash, err := u.SendTransaction(AddStakeTx, &address, amount, 0, nil)
	if err != nil {
		process.handleError(err, fmt.Sprintf("%v unable to send AddStakeTx, amount: %v", u.GetInfo(), amount))
	}
	log.Info(fmt.Sprintf("%v sent AddStakeTx, hash: %v, amount: %v", u.GetInfo(), hash, amount))
	if err := waitForMinedTransaction(u, hash, time.Minute*5); err != nil {
		process.handleError(err, fmt.Sprintf("%v unable to mine delegate tx", u.GetInfo()))
	}
	log.Info(fmt.Sprintf("%v mined AddStakeTx, hash: %v, amount: %v", u.GetInfo(), hash, amount))
}
