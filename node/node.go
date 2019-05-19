package node

import (
	"bufio"
	"fmt"
	"idena-test-go/log"
	"os"
	"os/exec"
	"strconv"
	"time"
)

const (
	dbFileName = "idenachain.db"

	argRpcPortName  = "--rpcport"
	argIpfsBootNode = "--ipfsbootnode"
	argGodAddress   = "--godaddress"
	argCeremonyTime = "--ceremonytime"
	argAutoMine     = "--automine"
	argBootNode     = "--bootnode"
	argIpfsPort     = "--ipfsport"
	argPort         = "--port"
	argDataDir      = "--datadir"
	verbosity       = "--verbosity"

	NodeStartWaitingTime = 5 * time.Second
	nodeStopWaitingTime  = 2 * time.Second
)

const (
	DeleteNothing = iota
	DeleteDataDir
	DeleteDb
)

type Node struct {
	workDir         string
	execCommandName string
	dataDir         string
	port            int
	autoMine        bool
	RpcPort         int
	BootNode        string
	IpfsBootNode    string
	ipfsPort        int
	GodAddress      string
	CeremonyTime    int64
	process         *os.Process
	logFile         *os.File
	verbosity       int
}

func NewNode(workDir string, execCommandName string, dataDir string, port int, autoMine bool, rpcPort int,
	bootNode string, ipfsBootNode string, ipfsPort int, godAddress string, ceremonyTime int64, verbosity int) *Node {
	return &Node{
		workDir:         workDir,
		execCommandName: execCommandName,
		dataDir:         dataDir,
		port:            port,
		autoMine:        autoMine,
		RpcPort:         rpcPort,
		BootNode:        bootNode,
		IpfsBootNode:    ipfsBootNode,
		ipfsPort:        ipfsPort,
		GodAddress:      godAddress,
		CeremonyTime:    ceremonyTime,
		verbosity:       verbosity,
	}
}

func (node *Node) Start(deleteMode int) {
	debug := true

	if node.process != nil {
		node.killProcess()
		time.Sleep(nodeStopWaitingTime)
	}
	if node.logFile != nil {
		node.logFile.Close()
		node.logFile = nil
	}

	if deleteMode == DeleteDataDir {
		node.deleteDataDir()
	} else if deleteMode == DeleteDb {
		node.deleteDb()
	}

	args := node.getArgs()
	command := exec.Command(node.workDir+string(os.PathSeparator)+node.execCommandName, args...)
	command.Dir = node.workDir
	if debug {
		f, err := os.Create(node.workDir + string(os.PathSeparator) + fmt.Sprintf("port-%v", node.ipfsPort))
		if err != nil {
			panic(err)
		}
		out := bufio.NewWriter(f)
		command.Stdout = out
		node.logFile = f
	}

	command.Start()
	node.process = command.Process
	time.Sleep(NodeStartWaitingTime)

	//if debug {
	//log.Debug(fmt.Sprintf("Node with rpc port %v start log:", node.RpcPort))
	//log.Debug(out.String())
	//log.Debug("-----------------------------------------")
	//}

	log.Info(fmt.Sprintf("Started node, workDir: %v, parameters: %v", node.workDir, args))
}

func (node *Node) Destroy() {
	if node.process != nil {
		node.killProcess()
	}
	if node.logFile != nil {
		node.logFile.Close()
		node.logFile = nil
	}
}

func (node *Node) killProcess() {
	err := node.process.Kill()
	if err != nil {
		panic(err)
	}
	node.process = nil
}

func (node *Node) deleteDataDir() {
	deleteDir(node.workDir + node.dataDir)
}

func (node *Node) deleteDb() {
	deleteDir(node.workDir + node.dataDir + string(os.PathSeparator) + dbFileName)
}

func deleteDir(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return
	}
	err := os.RemoveAll(path)
	if err != nil {
		panic(err)
	}
}

func (node *Node) getArgs() []string {
	var args []string

	if len(node.dataDir) > 0 {
		args = append(args, argDataDir)
		args = append(args, node.dataDir)
	}

	if node.autoMine {
		args = append(args, argAutoMine)
	}

	if len(node.GodAddress) > 0 {
		args = append(args, argGodAddress)
		args = append(args, node.GodAddress)
	}

	if node.CeremonyTime > 0 {
		args = append(args, argCeremonyTime)
		args = append(args, strconv.FormatInt(node.CeremonyTime, 10))
	}

	args = append(args, argBootNode)
	args = append(args, node.BootNode)

	if node.ipfsPort > 0 {
		args = append(args, argIpfsPort)
		args = append(args, strconv.Itoa(node.ipfsPort))
	}

	if node.RpcPort > 0 {
		args = append(args, argRpcPortName)
		args = append(args, strconv.Itoa(node.RpcPort))
	}

	if node.port > 0 {
		args = append(args, argPort)
		args = append(args, strconv.Itoa(node.port))
	}

	if len(node.IpfsBootNode) > 0 {
		args = append(args, argIpfsBootNode)
		args = append(args, node.IpfsBootNode)
	}

	args = append(args, verbosity)
	args = append(args, strconv.Itoa(node.verbosity))

	args = append(args, "--maxnetdelay")
	args = append(args, "500")

	return args
}
