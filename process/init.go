package process

import (
	"fmt"
	"github.com/idena-network/idena-go/common/eventbus"
	"github.com/idena-network/idena-test-go/client"
	"github.com/idena-network/idena-test-go/common"
	"github.com/idena-network/idena-test-go/events"
	"github.com/idena-network/idena-test-go/log"
	"github.com/idena-network/idena-test-go/node"
	"github.com/idena-network/idena-test-go/scenario"
	"github.com/idena-network/idena-test-go/user"
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (process *Process) init() {
	log.Debug("Start initializing")

	process.bus = eventbus.New()

	process.loadNodeBaseConfigData()

	if process.godMode {
		process.createGodUser()
		process.startGodNode()
	}

	process.initGodAddress()
	process.initIpfsBootNode()

	process.ceremonyTime = process.getCeremonyTime()

	if process.godMode {
		process.restartGodNode()
	}

	process.bus.Subscribe(events.NodeCrashedEventID, func(e eventbus.Event) {
		nodeCrashedEvent := e.(*events.NodeCrashedEvent)
		if nodeCrashedEvent.Index >= 500 {
			return
		}
		u := process.users[nodeCrashedEvent.Index]
		if !u.IsActive() {
			log.Warn(fmt.Sprintf("%v node will not be restarted due to crash since it is not active", u.GetInfo()))
			return
		}
		log.Warn(fmt.Sprintf("%v node will be restarted due to crash", u.GetInfo()))
		_ = u.DestroyNode()
		process.startNode(u, node.DeleteNothing)
		if !process.godMode {
			if err := process.apiClient.SendWarnNotification(fmt.Sprintf("%v node has been restarted due to crash", u.GetInfo())); err != nil {
				log.Error(errors.Wrap(err, "Unable to send warn notification to god bot").Error())
			}
		}
	})

	process.createExternalUsers()

	log.Debug("Initialization completed")
}

func (process *Process) loadNodeBaseConfigData() {
	if len(process.nodeBaseConfigFileName) == 0 {
		return
	}
	var byteValue []byte
	var err error
	if common.IsValidUrl(process.nodeBaseConfigFileName) {
		byteValue, err = common.LoadData(process.nodeBaseConfigFileName)
		if err != nil {
			panic(err)
		}
	} else {
		file, err := os.Open(filepath.Join(process.workDir, process.nodeBaseConfigFileName))
		if err != nil {
			panic(err)
		}

		byteValue, err = ioutil.ReadAll(file)
		if err != nil {
			panic(err)
		}
	}
	process.nodeBaseConfigData = byteValue
}

func isNodeShared(index int, sc scenario.Scenario) bool {
	for _, epochNewUsers := range sc.EpochNewUsersBeforeFlips {
		for _, newUsers := range epochNewUsers {
			if newUsers.SharedNode != nil && *newUsers.SharedNode == index {
				return true
			}
		}
	}
	for _, epochNewUsers := range sc.EpochNewUsersAfterFlips {
		for _, newUsers := range epochNewUsers {
			if newUsers.SharedNode != nil && *newUsers.SharedNode == index {
				return true
			}
		}
	}
	return false
}

func (process *Process) createGodUser() {
	index := 0
	apiKey := generateApiKey(index, process.randomApiKeys, process.predefinedApiKeys)
	profile := process.defineNewNodeProfile(true, isNodeShared(index, process.sc))
	if profile == lowPowerProfile {
		process.lowPowerProfileCount++
	}
	n := node.NewNode("",
		index,
		process.workDir,
		process.execCommandName,
		DataDir,
		getNodeDataDir(index, process.firstRpcPort+process.firstPortOffset+index),
		process.firstPort+process.firstPortOffset+index,
		true,
		process.rpcHost,
		process.firstRpcPort+process.firstPortOffset+index,
		"",
		process.firstIpfsPort+process.firstPortOffset+index,
		"",
		0,
		process.verbosity,
		process.maxNetDelay,
		process.nodeBaseConfigData,
		process.nodeStartWaitingTime,
		process.nodeStopWaitingTime,
		apiKey,
		profile,
		isNodeShared(index, process.sc),
	)
	u := user.NewUser(nil, client.NewClient(n.RpcPort, index, apiKey, process.bus), n, index)
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
		err := u.InitAddress()
		process.handleError(err, fmt.Sprintf("%v unable to get address", u.GetInfo()))
		godAddress := u.GetAddress()
		return godAddress
	}
	c := process.apiClient
	var err error
	for cnt := 10; cnt > 0; cnt-- {
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

func (process *Process) initIpfsBootNode() {
	process.ipfsBootNode = process.getIpfsBootNode()
	log.Info(fmt.Sprintf("Got ipfs boot node: %v", process.ipfsBootNode))
}

func (process *Process) getIpfsBootNode() string {
	if process.godMode {
		u := process.godUser
		ipfsBootNode, err := u.GetIpfsAddress()
		process.handleError(err, fmt.Sprintf("%v unable to get ipfs boot node", u.GetInfo()))
		ipfsBootNode = strings.Replace(ipfsBootNode, "0.0.0.0", "127.0.0.1", 1)
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
	u.UpdateNodeParameters(process.godAddress, process.ipfsBootNode, process.ceremonyTime)
	process.handleError(u.Start(node.DeleteDb), "Unable to start node")
	log.Info("Restarted god node")
}

func (process *Process) createExternalUsers() {
	index := 500
	for _, privateKey := range process.sc.ExternalUserKeys {
		rpcPort := process.firstRpcPort + process.firstPortOffset + index
		apiKey := generateApiKey(index, process.randomApiKeys, process.predefinedApiKeys)
		profile := ""
		n := node.NewNode(privateKey,
			index,
			process.workDir,
			process.execCommandName,
			DataDir,
			getNodeDataDir(index, rpcPort),
			process.firstPort+process.firstPortOffset+index,
			false,
			process.rpcHost,
			rpcPort,
			process.ipfsBootNode,
			process.firstIpfsPort+process.firstPortOffset+index,
			process.godAddress,
			process.ceremonyTime,
			process.verbosity,
			process.maxNetDelay,
			process.nodeBaseConfigData,
			process.nodeStartWaitingTime,
			process.nodeStopWaitingTime,
			apiKey,
			profile,
			isNodeShared(index, process.sc),
		)
		u := user.NewUser(nil, client.NewClient(rpcPort, index, apiKey, process.bus), n, index)
		process.externalUsers = append(process.externalUsers, u)
		log.Info(fmt.Sprintf("%v created as external", u.GetInfo()))
		index++
	}
}
