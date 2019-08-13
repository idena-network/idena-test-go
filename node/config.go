package node

import (
	"encoding/json"
	"fmt"
	"github.com/idena-network/idena-go/common"
	"github.com/idena-network/idena-go/config"
	"github.com/idena-network/idena-go/p2p/enode"
	"math/big"
	"path/filepath"
)

const network = 2

type specConfig struct {
	Network          *uint32 `json:"Network,omitempty"`
	DataDir          string
	P2P              p2pConfig
	RPC              rpcConfig
	GenesisConf      genesisConf
	Consensus        consensusConf
	IpfsConf         ipfsConfig
	Validation       *config.ValidationConfig
	Sync             *config.SyncConfig             `json:"Sync,omitempty"`
	OfflineDetection *config.OfflineDetectionConfig `json:"OfflineDetection,omitempty"`
	Blockchain       *config.BlockchainConfig       `json:"Blockchain,omitempty"`
}

type p2pConfig struct {
	ListenAddr     string
	BootstrapNodes []*enode.Node
	MaxDelay       int
}

type rpcConfig struct {
	HTTPHost string
	HTTPPort int
}

type genesisConf struct {
	Alloc             map[string]genesisAllocation
	FirstCeremonyTime int64
	GodAddress        string
}

type genesisAllocation struct {
	Balance *big.Int
	Stake   *big.Int
}

type consensusConf struct {
	Automine      bool
	SnapshotRange *uint64 `json:"SnapshotRange,omitempty"`
}

type ipfsConfig struct {
	DataDir   string
	BootNodes []string
	IpfsPort  int
}

func buildConfig(node *Node) interface{} {
	conf := node.buildSpecificConfig()
	if node.baseConfigData == nil {
		return conf
	}
	addConfig(conf, node.baseConfigData)
	return conf
}

func (node *Node) buildNodeConfigFileData() []byte {
	c := buildConfig(node)

	bytes, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}

	return bytes
}

func addConfig(c *specConfig, configData []byte) {
	if configData == nil {
		return
	}
	err := json.Unmarshal(configData, c)
	if err != nil {
		panic(err)
	}
}

func (node *Node) buildSpecificConfig() *specConfig {
	var nodes []*enode.Node
	if len(node.BootNode) > 0 {
		p, err := enode.ParseV4(node.BootNode)
		if err != nil {
			panic(err)
		}
		nodes = append(nodes, p)
	}

	godAddress := node.GodAddress
	if len(godAddress) == 0 {
		godAddress = config.DefaultGodAddress
	}

	dataDir := filepath.Join(node.dataDir, node.nodeDataDir)

	var bootNodes []string
	if len(node.IpfsBootNode) > 0 {
		bootNodes = []string{node.IpfsBootNode}
	}

	nw := uint32(network)
	return &specConfig{
		Network: &nw,
		DataDir: dataDir,
		P2P: p2pConfig{
			ListenAddr:     fmt.Sprintf(":%d", node.port),
			BootstrapNodes: nodes,
			MaxDelay:       node.maxNetDelay,
		},
		RPC: rpcConfig{
			HTTPHost: node.RpcHost,
			HTTPPort: node.RpcPort,
		},
		GenesisConf: genesisConf{
			Alloc: map[string]genesisAllocation{
				godAddress: {
					Balance: big.NewInt(0).Mul(common.DnaBase, big.NewInt(99999)),
					Stake:   big.NewInt(0).Mul(common.DnaBase, big.NewInt(9999)),
				},
			},
			GodAddress:        godAddress,
			FirstCeremonyTime: node.CeremonyTime,
		},
		Consensus: consensusConf{
			Automine: node.autoMine,
		},

		IpfsConf: ipfsConfig{
			DataDir:   filepath.Join(dataDir, config.DefaultIpfsDataDir),
			IpfsPort:  node.ipfsPort,
			BootNodes: bootNodes,
		},
	}
}
