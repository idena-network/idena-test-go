package node

import (
	"bufio"
	"fmt"
	"github.com/idena-network/idena-test-go/log"
	"github.com/pkg/errors"
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

	StartWaitingTime = 10 * time.Second
	StopWaitingTime  = 4 * time.Second
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

func (node *Node) Start(deleteMode StartMode) error {
	if deleteMode == DeleteDataDir {
		if err := node.deleteDataDir(); err != nil {
			return err
		}
	} else if deleteMode == DeleteDb {
		if err := node.deleteDb(); err != nil {
			return err
		}
	}

	if err := node.deleteConfigFile(); err != nil {
		return err
	}
	if err := node.createConfigFile(); err != nil {
		return err
	}

	args := node.getArgs()
	command := exec.Command(filepath.Join(node.workDir, node.execCommandName), args...)
	command.Dir = node.workDir

	filePath := filepath.Join(node.workDir, node.dataDir, fmt.Sprintf("node-%d-%d.log", node.index, node.RpcPort))
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return errors.Wrapf(err, "unable to init node log file")
	}

	out := bufio.NewWriter(f)
	command.Stdout = out
	command.Stderr = out
	node.logWriter = out

	if err = command.Start(); err != nil {
		return errors.Wrapf(err, "unable to start node process")
	}
	node.process = command.Process
	time.Sleep(StartWaitingTime)

	log.Info(fmt.Sprintf("Started node, workDir: %v, parameters: %v", node.workDir, args))
	return nil
}

func (node *Node) Stop() error {
	if err := node.Destroy(); err != nil {
		return err
	}
	time.Sleep(StopWaitingTime)
	return nil
}

func (node *Node) Destroy() error {
	if node.logWriter != nil {
		node.logWriter.Flush()
		node.logWriter = nil
	}
	if node.process != nil {
		return node.killProcess()
	}
	return nil
}

func (node *Node) killProcess() error {
	err := node.process.Kill()
	if err != nil {
		return errors.Wrapf(err, "unable to kill node process")
	}
	node.process = nil
	log.Info("Killed node process")
	return nil
}

func (node *Node) deleteDataDir() error {
	return removeFile(filepath.Join(node.workDir, node.dataDir, node.nodeDataDir))
}

func (node *Node) deleteDb() error {
	return removeFile(filepath.Join(node.workDir, node.dataDir, node.nodeDataDir, dbFileName))
}

func (node *Node) deleteConfigFile() error {
	return removeFile(node.getConfigFileFullName())
}

func (node *Node) createConfigFile() error {
	f, err := os.OpenFile(node.getConfigFileFullName(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return errors.Wrapf(err, "unable to init node config file")
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	_, err = w.Write(node.buildNodeConfigFileData())
	if err != nil {
		return errors.Wrapf(err, "unable to fill node config file")
	}
	w.Flush()
	return nil
}

func (node *Node) getConfigFileFullName() string {
	return filepath.Join(node.workDir, node.dataDir, node.getConfigFileName())
}

func (node *Node) getConfigFileName() string {
	return fmt.Sprintf("config-%d-%d.json", node.index, node.RpcPort)
}

func removeFile(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	return errors.Wrapf(os.RemoveAll(path), "unable to remove file")
}

func (node *Node) getArgs() []string {
	var args []string

	args = append(args, argConfigFile)
	args = append(args, filepath.Join(node.dataDir, node.getConfigFileName()))

	args = append(args, verbosity)
	args = append(args, strconv.Itoa(node.verbosity))

	return args
}
