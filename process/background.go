package process

import (
	"fmt"
	"github.com/idena-network/idena-go/blockchain/types"
	"github.com/idena-network/idena-test-go/common"
	"github.com/idena-network/idena-test-go/log"
	"github.com/idena-network/idena-test-go/user"
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

		common.WaitWithTimeout(wg, timeout)
		log.Debug("Epoch background process completed")
		flag.completed = true
	}()
	log.Debug("Epoch background process started")
}

func (process *Process) generateTxs(flag *EpochCompletionFlag) {
	testIndex := process.getCurrentTestIndex()
	txs, ok := process.sc.EpochTxs[testIndex]
	if !ok {
		return
	}
	activeUsers := process.filterActiveUsers(txs.Users)
	if len(activeUsers) < 2 {
		log.Warn(fmt.Sprintf("Not enough active users for generating transactions: %d", len(activeUsers)))
		return
	}
	log.Info(fmt.Sprintf("Start generating txs (test #%d)", testIndex))
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
		amount := float32(0.01)
		hash, err := sender.Client.SendTransaction(types.SendTx, sender.Address, recipient.Address, amount, 1.0, nil)
		if err != nil {
			log.Error(fmt.Sprintf("Unable to send transaction (amount=%f) from %s to %s: %v", amount, sender.GetInfo(), recipient.GetInfo(), err))
		}
		log.Debug(fmt.Sprintf("Sent transaction from %s to %s, hash: %s", sender.GetInfo(), recipient.GetInfo(), hash))

		time.Sleep(txs.Period)
	}
	log.Info(fmt.Sprintf("Stop generating txs (test #%d)", testIndex))
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
