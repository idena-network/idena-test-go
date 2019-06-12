package process

import (
	"errors"
	"fmt"
	"idena-test-go/log"
	"strings"
)

func (process *Process) GetGodAddress() (string, error) {
	if len(process.godAddress) == 0 {
		return "", errors.New("god address has not been initialized yet")
	}
	return process.godAddress, nil
}

func (process *Process) GetBootNode() (string, error) {
	if len(process.bootNode) == 0 {
		return "", errors.New("boot node has not been initialized yet")
	}
	return process.toExternal(process.bootNode), nil
}

func (process *Process) GetIpfsBootNode() (string, error) {
	if len(process.ipfsBootNode) == 0 {
		return "", errors.New("ipfs boot node has not been initialized yet")
	}
	return process.toExternal(process.ipfsBootNode), nil
}

func (process *Process) RequestInvite(address string) error {
	if !process.godMode {
		return errors.New("unable to send invite as there is no god node")
	}
	if process.godUser == nil {
		return errors.New("god node has not been initialized yet")
	}
	invite, err := process.godUser.Client.SendInvite(address)
	if err != nil {
		return err
	}
	log.Info(fmt.Sprintf("%s sent invite %s to %s by request", process.godUser.GetInfo(), invite.Hash, address))
	return nil
}

func (process *Process) GetCeremonyTime() (int64, error) {
	if process.ceremonyTime == 0 {
		return 0, errors.New("ceremony time has not been initialized yet")
	}
	return process.ceremonyTime, nil
}

func (process *Process) toExternal(host string) string {
	external := strings.Replace(host, "localhost", process.godHost, 1)
	external = strings.Replace(external, "127.0.0.1", process.godHost, 1)
	return external
}
