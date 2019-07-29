package node

import (
	"bufio"
	"fmt"
	"github.com/idena-network/idena-test-go/log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

const (
	dbFileName = "idenachain.db"

	argConfigFile = "--config"
	verbosity     = "--verbosity"

	StartWaitingTime = 5 * time.Second
	StopWaitingTime  = 2 * time.Second
)

type StartMode int

const (
	DeleteNothing StartMode = iota
	DeleteDataDir
	DeleteDb
)

type Node struct {
	index           int
	workDir         string
	execCommandName string
	dataDir         string
	nodeDataDir     string
	port            int
	autoMine        bool
	RpcHost         string
	RpcPort         int
	BootNode        string
	IpfsBootNode    string
	ipfsPort        int
	GodAddress      string
	CeremonyTime    int64
	process         *os.Process
	logWriter       *bufio.Writer
	verbosity       int
	maxNetDelay     int
	baseConfigData  []byte
}

func NewNode(index int, workDir string, execCommandName string, dataDir string, nodeDataDir string, port int,
	autoMine bool, rpcHost string, rpcPort int, bootNode string, ipfsBootNode string, ipfsPort int, godAddress string,
	ceremonyTime int64, verbosity int, maxNetDelay int, baseConfigData []byte) *Node {

	return &Node{
		index:           index,
		workDir:         workDir,
		execCommandName: execCommandName,
		dataDir:         dataDir,
		nodeDataDir:     nodeDataDir,
		port:            port,
		autoMine:        autoMine,
		RpcHost:         rpcHost,
		RpcPort:         rpcPort,
		BootNode:        bootNode,
		IpfsBootNode:    ipfsBootNode,
		ipfsPort:        ipfsPort,
		GodAddress:      godAddress,
		CeremonyTime:    ceremonyTime,
		verbosity:       verbosity,
		maxNetDelay:     maxNetDelay,
		baseConfigData:  baseConfigData,
	}
}

func (node *Node) Start(deleteMode StartMode) {

	if deleteMode == DeleteDataDir {
		node.deleteDataDir()
	} else if deleteMode == DeleteDb {
		node.deleteDb()
	}

	node.deleteConfigFile()
	node.createConfigFile()

	args := node.getArgs()
	command := exec.Command(filepath.Join(node.workDir, node.execCommandName), args...)
	command.Dir = node.workDir

	filePath := filepath.Join(node.workDir, node.dataDir, fmt.Sprintf("node-%d-%d.log", node.index, node.RpcPort))
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}

	out := bufio.NewWriter(f)
	command.Stdout = out
	node.logWriter = out

	command.Start()
	node.process = command.Process
	time.Sleep(StartWaitingTime)

	log.Info(fmt.Sprintf("Started node, workDir: %v, parameters: %v", node.workDir, args))
}

func (node *Node) Stop() {
	node.Destroy()
	time.Sleep(StopWaitingTime)
}

func (node *Node) Destroy() {
	if node.logWriter != nil {
		node.logWriter.Flush()
		node.logWriter = nil
	}
	if node.process != nil {
		node.killProcess()
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
	removeFile(filepath.Join(node.workDir, node.dataDir, node.nodeDataDir))
}

func (node *Node) deleteDb() {
	removeFile(filepath.Join(node.workDir, node.dataDir, node.nodeDataDir, dbFileName))
}

func (node *Node) deleteConfigFile() {
	removeFile(node.getConfigFileFullName())
}

func (node *Node) createConfigFile() {
	f, err := os.OpenFile(node.getConfigFileFullName(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	_, err = w.Write(node.buildNodeConfigFileData())
	if err != nil {
		panic(err)
	}
	w.Flush()
}

func (node *Node) getConfigFileFullName() string {
	return filepath.Join(node.workDir, node.dataDir, node.getConfigFileName())
}

func (node *Node) getConfigFileName() string {
	return fmt.Sprintf("config-%d-%d.json", node.index, node.RpcPort)
}

func removeFile(path string) {
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

	args = append(args, argConfigFile)
	args = append(args, filepath.Join(node.dataDir, node.getConfigFileName()))

	args = append(args, verbosity)
	args = append(args, strconv.Itoa(node.verbosity))

	return args
}
