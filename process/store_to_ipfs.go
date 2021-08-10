package process

import (
	"fmt"
	"github.com/google/tink/go/subtle/random"
	"github.com/idena-network/idena-test-go/log"
	"github.com/idena-network/idena-test-go/user"
	"time"
)

func (process *Process) sendStoreToIpfsTxs(u user.User) {
	epoch := process.getCurrentTestIndex()
	epochStoreToIpfsTxs, ok := process.sc.StoreToIpfsTxs[epoch]
	if !ok {
		return
	}
	userStoreToIpfsTxs, ok := epochStoreToIpfsTxs[u.GetIndex()]
	if !ok {
		return
	}
	time.Sleep(userStoreToIpfsTxs.Delay)
	for i, size := range userStoreToIpfsTxs.Sizes {
		if i > 0 {
			time.Sleep(userStoreToIpfsTxs.StepDelay)
		}
		process.sendStoreToIpfsTx(u, size)
	}
}

func (process *Process) sendStoreToIpfsTx(u user.User, size int) {
	data := random.GetRandomBytes(uint32(size))
	dataHex := toHex(data)
	cid, err := u.AddIpfsData(dataHex, true)
	if err != nil {
		process.handleError(err, fmt.Sprintf("%v unable to add ipfs data", u.GetInfo()))
	}
	log.Info(fmt.Sprintf("%v added ipfs data to send StoreToIpfsTx", u.GetInfo()))
	hash, err := u.StoreToIpfs(cid)
	if err != nil {
		process.handleError(err, fmt.Sprintf("%v unable to send StoreToIpfsTx, cid: %v", u.GetInfo(), cid))
	}
	log.Info(fmt.Sprintf("%v sent StoreToIpfsTx, hash: %v", u.GetInfo(), hash))
	if err := waitForMinedTransaction(u, hash, time.Minute*5); err != nil {
		process.handleError(err, fmt.Sprintf("%v unable to mine StoreToIpfsTx", u.GetInfo()))
	}
	log.Info(fmt.Sprintf("%v mined StoreToIpfsTx, hash: %v", u.GetInfo(), hash))
}
