package process

import (
	"fmt"
	"github.com/idena-network/idena-test-go/client"
	"github.com/idena-network/idena-test-go/log"
	"github.com/idena-network/idena-test-go/node"
	"github.com/idena-network/idena-test-go/user"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

func (process *Process) init() {
	log.Debug("Start initializing")

	process.loadNodeBaseConfigData()

	if process.godMode {
		process.createGodUser()
		process.startGodNode()
	}

	process.initGodAddress()
	process.initBootNode()
	process.initIpfsBootNode()

	process.ceremonyTime = process.getCeremonyTime()

	if process.godMode {
		process.restartGodNode()
	}

	log.Debug("Initialization completed")
}

func (process *Process) loadNodeBaseConfigData() {
	if len(process.nodeBaseConfigFileName) == 0 {
		return
	}
	file, err := os.Open(filepath.Join(process.workDir, process.nodeBaseConfigFileName))
	if err != nil {
		panic(err)
	}

	byteValue, err := ioutil.ReadAll(file)
	if err != nil {
		panic(err)
	}
	process.nodeBaseConfigData = byteValue
}

func (process *Process) createGodUser() {
	index := 0
	n := node.NewNode(index,
		process.workDir,
		process.execCommandName,
		DataDir,
		getNodeDataDir(index, firstRpcPort+process.firstPortOffset+index),
		firstPort+process.firstPortOffset+index,
		true,
		process.rpcHost,
		firstRpcPort+process.firstPortOffset+index,
		"",
		"",
		firstIpfsPort+process.firstPortOffset+index,
		"",
		0,
		process.verbosity,
		process.maxNetDelay,
		process.nodeBaseConfigData,
	)
	u := user.NewUser(client.NewClient(*n, process.reqIdHolder), n, index)
	process.godUser = u
	process.users = append(process.users, u)
	log.Info("Created god user")
}

func (process *Process) startGodNode() {
	process.handleError(process.godUser.Start(node.DeleteDataDir), "Unable to start god node")
	log.Info("Started god node")
}

func (process *Process) initGodAddress() {
	process.godAddress = process.getGodAddress()
	log.Info(fmt.Sprintf("Got god address: %v", process.godAddress))
}

func (process *Process) getGodAddress() string {
	if process.godMode {
		u := process.godUser
		godAddress, err := u.Client.GetCoinbaseAddr()
		process.handleError(err, fmt.Sprintf("%v unable to get address", u.GetInfo()))
		return godAddress
	}
	c := process.apiClient
	var err error
	for cnt := 5; cnt > 0; cnt-- {
		var godAddress string
		godAddress, err = c.GetGodAddress()
		if err == nil {
			return godAddress
		}
		time.Sleep(requestRetryDelay)
	}
	process.handleError(err, "Unable to get god node address from god bot")
	return ""
}

func (process *Process) initBootNode() {
	process.bootNode = process.getBootNode()
	log.Info(fmt.Sprintf("Got boot node enode: %v", process.bootNode))
}

func (process *Process) getBootNode() string {
	if process.godMode {
		u := process.godUser
		bootNode, err := u.Client.GetEnode()
		process.handleError(err, fmt.Sprintf("%v unable to get enode", u.GetInfo()))
		return bootNode
	}
	c := process.apiClient
	bootNode, err := c.GetBootNode()
	process.handleError(err, "Unable to get boot node")
	return bootNode
}

func (process *Process) initIpfsBootNode() {
	process.ipfsBootNode = process.getIpfsBootNode()
	log.Info(fmt.Sprintf("Got ipfs boot node: %v", process.ipfsBootNode))
}

func (process *Process) getIpfsBootNode() string {
	if process.godMode {
		u := process.godUser
		ipfsBootNode, err := u.Client.GetIpfsAddress()
		process.handleError(err, fmt.Sprintf("%v unable to get ipfs boot node", u.GetInfo()))
		return ipfsBootNode
	}
	c := process.apiClient
	ipfsBootNode, err := c.GetIpfsBootNode()
	process.handleError(err, "Unable to get ipfs boot node")
	return ipfsBootNode
}

func (process *Process) getCeremonyTime() int64 {
	if process.godMode {
		return time.Now().UTC().Unix() + int64(process.sc.CeremonyMinOffset*60)
	}
	ceremonyTime, err := process.apiClient.GetCeremonyTime()
	if err != nil {
		process.handleError(err, "Unable to get ceremony time")
	}
	return ceremonyTime
}

func (process *Process) restartGodNode() {
	u := process.godUser
	process.handleError(u.Stop(), "Unable to stop node")
	u.Node.BootNode = process.bootNode
	u.Node.GodAddress = process.godAddress
	u.Node.IpfsBootNode = process.ipfsBootNode
	u.Node.CeremonyTime = process.ceremonyTime
	process.handleError(u.Start(node.DeleteDb), "Unable to start node")
	log.Info("Restarted god node")
}
