package process

import (
	"errors"
	"fmt"
	"github.com/idena-network/idena-test-go/log"
	"strings"
	"time"
)

func (process *Process) GetGodAddress() (string, error) {
	if len(process.godAddress) == 0 {
		return "", errors.New("god address has not been initialized yet")
	}
	return process.godAddress, nil
}

func (process *Process) GetIpfsBootNode() (string, error) {
	if len(process.ipfsBootNode) == 0 {
		return "", errors.New("ipfs boot node has not been initialized yet")
	}
	return process.toExternal(process.ipfsBootNode), nil
}

func (process *Process) RequestInvites(addresses []string) error {
	if !process.godMode {
		return errors.New("unable to send invites as there is no god node")
	}
	if process.godUser == nil {
		return errors.New("god node has not been initialized yet")
	}
	for _, address := range addresses {
		invite, err := process.godUser.Client.SendInvite(address, process.sc.InviteAmount)
		if err != nil {
			log.Error(fmt.Sprintf("%s unable to send invite to %s by request: %v", process.godUser.GetInfo(), address, err))
			continue
		}
		log.Info(fmt.Sprintf("%s sent invite %s to %s by request", process.godUser.GetInfo(), invite.Hash, address))
	}
	return nil
}

func (process *Process) GetCeremonyTime() (int64, error) {
	if process.ceremonyTime == 0 {
		return 0, errors.New("ceremony time has not been initialized yet")
	}
	return process.ceremonyTime, nil
}

func (process *Process) GetEpoch() (uint16, error) {
	return uint16(process.getCurrentTestIndex()), nil
}

func (process *Process) SendFailNotification(message string, sender string) {
	go func() {
		time.Sleep(time.Second * 2)
		if process.allowFailNotification || process.validationOnly {
			log.Warn(fmt.Sprintf("Got fail notification from %s: %s", sender, message))
		} else {
			process.handleError(errors.New(message), fmt.Sprintf("Got fail notification from %s", sender))
		}
	}()
}

func (process *Process) SendWarnNotification(message string, sender string) {
	process.handleWarn(fmt.Sprintf("Got warn notification from %s: %s", sender, message))
}

func (process *Process) toExternal(host string) string {
	external := strings.Replace(host, "localhost", process.godHost, 1)
	external = strings.Replace(external, "0.0.0.0", process.godHost, 1)
	external = strings.Replace(external, "127.0.0.1", process.godHost, 1)
	return external
}
