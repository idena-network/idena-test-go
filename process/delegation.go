package process

import (
	"fmt"
	"github.com/idena-network/idena-test-go/log"
	"github.com/idena-network/idena-test-go/user"
	"sync"
	"time"
)

func (process *Process) delegate(u *user.User) {
	epoch := process.getCurrentTestIndex()
	epochDelegations, ok := process.sc.Delegations[epoch]
	if !ok {
		return
	}
	delegateeIdx, ok := epochDelegations[u.Index]
	if !ok {
		return
	}
	delegateeAddress := process.users[delegateeIdx].Address
	process.delegateTo(u, delegateeAddress)
}

func (process *Process) undelegate(u *user.User) {
	epoch := process.getCurrentTestIndex()
	epochUndelegations, ok := process.sc.Undelegations[epoch]
	if !ok {
		return
	}
	ok = false
	for _, uIndex := range epochUndelegations {
		if u.Index == uIndex {
			ok = true
			break
		}
	}
	if !ok {
		return
	}
	const UndelegateTx = 0x13
	hash, err := u.Client.SendTransaction(UndelegateTx, u.Address, nil, 0, 0, nil)
	if err != nil {
		process.handleError(err, fmt.Sprintf("%v unable to send undelegate tx", u.GetInfo()))
	}
	log.Info(fmt.Sprintf("%v sent undelegate tx, hash: %v", u.GetInfo(), hash))
	if err := waitForMinedTransaction(u.Client, hash, time.Minute*5); err != nil {
		process.handleError(err, fmt.Sprintf("%v unable to mine undelegate tx", u.GetInfo()))
	}
	log.Info(fmt.Sprintf("%v mined undelegate tx, hash: %v", u.GetInfo(), hash))
}

func (process *Process) delegateTo(u *user.User, delegateeAddress string) {
	const DelegateTx = 0x12
	hash, err := u.Client.SendTransaction(DelegateTx, u.Address, &delegateeAddress, 0, 0, nil)
	if err != nil {
		process.handleError(err, fmt.Sprintf("%v unable to send delegate tx to %v", u.GetInfo(), delegateeAddress))
	}
	log.Info(fmt.Sprintf("%v sent delegate tx to %v, hash: %v", u.GetInfo(), delegateeAddress, hash))
	if err := waitForMinedTransaction(u.Client, hash, time.Minute*5); err != nil {
		process.handleError(err, fmt.Sprintf("%v unable to mine delegate tx", u.GetInfo()))
	}
	log.Info(fmt.Sprintf("%v mined delegate tx to %v, hash: %v", u.GetInfo(), delegateeAddress, hash))
}

func (process *Process) killDelegators(u *user.User) {
	epoch := process.getCurrentTestIndex()
	epochKillDelegators, ok := process.sc.KillDelegators[epoch]
	if !ok {
		return
	}
	killDelegators, ok := epochKillDelegators[u.Index]
	if !ok {
		return
	}
	const KillDelegatorTx = 0x14
	wg := &sync.WaitGroup{}
	wg.Add(len(killDelegators))
	for _, delegatorToKill := range killDelegators {
		func(delegatorToKill int) {
			to := process.users[delegatorToKill].Address
			hash, err := u.Client.SendTransaction(KillDelegatorTx, u.Address, &to, 0, 0, nil)
			if err != nil {
				process.handleError(err, fmt.Sprintf("%v unable to send kill delegator tx to %v", to, u.GetInfo()))
			}
			log.Info(fmt.Sprintf("%v sent kill delegator tx, hash: %v", u.GetInfo(), hash))
			if err := waitForMinedTransaction(u.Client, hash, time.Minute*5); err != nil {
				process.handleError(err, fmt.Sprintf("%v unable to mine kill delegator tx to %v", u.GetInfo(), to))
			}
			log.Info(fmt.Sprintf("%v mined kill delegator tx to %v, hash: %v", u.GetInfo(), to, hash))
		}(delegatorToKill)
	}
	wg.Wait()
	log.Info(fmt.Sprintf("%v mined all kill delegator txs", u.GetInfo()))
}
