package process

import (
	"errors"
	"fmt"
	"idena-test-go/common"
	"idena-test-go/log"
	"idena-test-go/user"
	"sync"
	"time"
)

type EpochCompletionFlag struct {
	completed bool
}

func (process *Process) startEpochBackgroundProcess(wg *sync.WaitGroup, timeout time.Duration) {
	go func() {
		flag := &EpochCompletionFlag{}

		go process.generateTxs(flag)

		ok := common.WaitWithTimeout(wg, timeout)
		if !ok {
			process.handleError(errors.New("verification sessions timeout"), "")
		}
		log.Debug("Epoch background process completed")
		flag.completed = true
	}()
	log.Debug("Epoch background process started")
}

func (process *Process) generateTxs(flag *EpochCompletionFlag) {
	epoch := process.getCurrentTestEpoch()
	txs, ok := process.sc.EpochTxs[epoch]
	if !ok {
		return
	}
	activeUsers := process.filterActiveUsers(txs.Users)
	if len(activeUsers) < 2 {
		log.Warn(fmt.Sprintf("Not enough active users for generating transactions: %d", len(activeUsers)))
		return
	}
	log.Info(fmt.Sprintf("Start generating txs (test #%d)", epoch))
	senderIdx := -1
	for !flag.completed {
		senderIdx++
		if senderIdx == len(activeUsers) {
			senderIdx = 0
		}
		recipientIdx := senderIdx + 1
		if recipientIdx == len(activeUsers) {
			recipientIdx = 0
		}

		sender := activeUsers[senderIdx]
		recipient := activeUsers[recipientIdx]
		amount := float32(1.0)
		hash, err := sender.Client.SendTransaction(sender.Address, recipient.Address, amount)
		if err != nil {
			log.Error(fmt.Sprintf("Unable to send transaction (amount=%f) from %s to %s: %v", amount, sender.GetInfo(), recipient.GetInfo(), err))
		}
		log.Debug(fmt.Sprintf("Sent transaction from %s to %s, hash: %s", sender.GetInfo(), recipient.GetInfo(), hash))

		time.Sleep(txs.Period)
	}
	log.Info(fmt.Sprintf("Stop generating txs (test #%d)", epoch))
}

func (process *Process) filterActiveUsers(users []int) []*user.User {
	var activeUsers []*user.User
	for _, userIdx := range users {
		u := process.users[userIdx]
		if u.Active {
			activeUsers = append(activeUsers, u)
		}
	}
	return activeUsers
}
