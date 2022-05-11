package process

import (
	"fmt"
	"github.com/idena-network/idena-test-go/log"
	"github.com/idena-network/idena-test-go/user"
	"sync"
	"time"
)

func (process *Process) kill(u user.User) {
	epoch := process.getCurrentTestIndex()
	epochKills, ok := process.sc.Kills[epoch]
	if !ok {
		return
	}
	ok = false
	for _, uIndex := range epochKills {
		if u.GetIndex() == uIndex {
			ok = true
			break
		}
	}
	if !ok {
		return
	}
	const KillTx = 0x3
	hash, err := u.SendTransaction(KillTx, nil, 0, 0, nil)
	if err != nil {
		process.handleError(err, fmt.Sprintf("%v unable to send kill tx", u.GetInfo()))
	}
	log.Info(fmt.Sprintf("%v sent kill tx, hash: %v", u.GetInfo(), hash))
	if err := waitForMinedTransaction(u, hash, time.Minute*5); err != nil {
		process.handleError(err, fmt.Sprintf("%v unable to mine kill tx", u.GetInfo()))
	}
	log.Info(fmt.Sprintf("%v mined kill tx, hash: %v", u.GetInfo(), hash))
}

func (process *Process) killInvitees(u user.User) {
	epoch := process.getCurrentTestIndex()
	epochKillInvitees, ok := process.sc.KillInvitees[epoch]
	if !ok {
		return
	}
	killInvitees, ok := epochKillInvitees[u.GetIndex()]
	if !ok {
		return
	}
	const KillInviteeTx = 0xA
	wg := &sync.WaitGroup{}
	wg.Add(len(killInvitees))
	for _, inviteeToKill := range killInvitees {
		func(inviteeToKill int) {
			to := process.users[inviteeToKill].GetAddress()
			hash, err := u.SendTransaction(KillInviteeTx, &to, 0, 0, nil)
			if err != nil {
				process.handleError(err, fmt.Sprintf("%v unable to send KillInviteeTx to %v", to, u.GetInfo()))
			}
			log.Info(fmt.Sprintf("%v sent KillInviteeTx, hash: %v", u.GetInfo(), hash))
			if err := waitForMinedTransaction(u, hash, time.Minute*5); err != nil {
				process.handleError(err, fmt.Sprintf("%v unable to mine KillInviteeTx to %v", u.GetInfo(), to))
			}
			log.Info(fmt.Sprintf("%v mined KillInviteeTx to %v, hash: %v", u.GetInfo(), to, hash))
		}(inviteeToKill)
	}
	wg.Wait()
	log.Info(fmt.Sprintf("%v mined all KillInviteeTxs", u.GetInfo()))
}
