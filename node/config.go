package node

import (
	"encoding/json"
	"fmt"
	"github.com/idena-network/idena-go/config"
	"github.com/idena-network/idena-go/p2p/enode"
	"path/filepath"
)

type specConfig struct {
	DataDir     string
	P2P         p2pConfig
	RPC         rpcConfig
	GenesisConf genesisConf
	Consensus   consensusConf
	IpfsConf    ipfsConfig
	Validation  *config.ValidationConfig
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
	FirstCeremonyTime int64
	GodAddress        string
}

type consensusConf struct {
	Automine bool
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
	return &specConfig{
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
			GodAddress:        godAddress,
			FirstCeremonyTime: node.CeremonyTime,
		},
		Consensus: consensusConf{
			Automine: node.autoMine,
		},

		IpfsConf: ipfsConfig{
			DataDir:   filepath.Join(dataDir, config.DefaultIpfsDataDir),
			IpfsPort:  node.ipfsPort,
			BootNodes: []string{node.IpfsBootNode},
		},
	}
}
