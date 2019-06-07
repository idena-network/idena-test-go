package node

import (
	"encoding/json"
	"fmt"
	"idena-go/common"
	"idena-go/config"
	"idena-go/p2p"
	"idena-go/p2p/enode"
	"idena-go/p2p/nat"
	"idena-go/rpc"
	"path/filepath"
)

type specConfig struct {
	DataDir     string
	P2P         p2pConfig
	RPC         rpcConfig
	GenesisConf genesisConf
	Consensus   consensusConf
	IpfsConf    ipfsConfig
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
	specConfig := node.buildSpecificConfig()
	if node.baseConfigData == nil {
		return specConfig
	}
	totalConfig := buildDefaultConfig()
	addConfig(totalConfig, node.baseConfigData)
	addSpecificConfig(totalConfig, specConfig)
	return totalConfig
}

func (node *Node) buildNodeConfigFileData() []byte {
	c := buildConfig(node)

	bytes, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}

	return bytes
}

func buildDefaultConfig() *config.Config {
	return &config.Config{
		P2P: &p2p.Config{
			MaxPeers: 25,
			NAT:      nat.Any(),
		},
		Consensus: config.GetDefaultConsensusConfig(),
		RPC:       rpc.GetDefaultRPCConfig(config.DefaultRpcHost, config.DefaultRpcPort),
		GenesisConf: &config.GenesisConf{
			FirstCeremonyTime: config.DefaultCeremonyTime,
			GodAddress:        common.HexToAddress(config.DefaultGodAddress),
		},
		IpfsConf: &config.IpfsConfig{
			DataDir:   filepath.Join(config.DefaultDataDir, config.DefaultIpfsDataDir),
			IpfsPort:  config.DefaultIpfsPort,
			BootNodes: config.DefaultIpfsBootstrapNodes,
			SwarmKey:  config.DefaultSwarmKey,
		},
		Validation: config.GetDefaultValidationConfig(),
	}
}

func addConfig(c *config.Config, configData []byte) {
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

func addSpecificConfig(totalConfig *config.Config, specConfig *specConfig) {
	specBytes, err := json.Marshal(specConfig)
	if err != nil {
		panic(err)
	}
	addConfig(totalConfig, specBytes)
}
