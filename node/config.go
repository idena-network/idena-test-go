package node

import (
	"encoding/json"
	"github.com/idena-network/idena-go/common"
	"github.com/idena-network/idena-go/config"
	"math"
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
	Mempool          *config.Mempool                `json:"Mempool,omitempty"`
}

type p2pConfig struct {
	MaxDelay       *int
	CollectMetrics bool
}

type rpcConfig struct {
	HTTPHost string
	HTTPPort int
}

type genesisConf struct {
	Alloc             map[string]genesisAllocation
	FirstCeremonyTime int64
	GodAddress        string
	GodAddressInvites uint16
}

type genesisAllocation struct {
	Balance *big.Int
	Stake   *big.Int
}

type consensusConf struct {
	Automine             bool
	SnapshotRange        *uint64  `json:"SnapshotRange,omitempty"`
	StatusSwitchRange    *uint64  `json:"StatusSwitchRange,omitempty"`
	MinProposerThreshold *float64 `json:"MinProposerThreshold,omitempty"`
}

type ipfsConfig struct {
	DataDir     string
	BootNodes   []string
	IpfsPort    int
	LowWater    *int
	HighWater   *int
	GracePeriod *string
	Profile     *string
}

func buildConfig(node *Node) interface{} {
	conf := node.buildSpecificConfig()
	if node.baseConfigData == nil {
		return conf
	}
	return addConfig(conf, node.baseConfigData)
}

func (node *Node) buildNodeConfigFileData() []byte {
	c := buildConfig(node)

	bytes, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}

	return bytes
}

func addConfig(c *specConfig, configData []byte) interface{} {
	if configData == nil {
		return c
	}
	cData, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	cMap := make(map[string]interface{})
	if err := json.Unmarshal(cData, &cMap); err != nil {
		panic(err)
	}
	cMap["GenesisConf"].(map[string]interface{})["Alloc"] = c.GenesisConf.Alloc
	if err := json.Unmarshal(configData, &cMap); err != nil {
		panic(err)
	}
	return cMap
}

func (node *Node) buildSpecificConfig() *specConfig {

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
	maxNetDelay := node.maxNetDelay
	return &specConfig{
		Network: &nw,
		DataDir: dataDir,
		P2P: p2pConfig{
			MaxDelay: &maxNetDelay,
		},
		RPC: rpcConfig{
			HTTPHost: node.RpcHost,
			HTTPPort: node.RpcPort,
		},
		GenesisConf: genesisConf{
			Alloc: map[string]genesisAllocation{
				godAddress: {
					Balance: big.NewInt(0).Mul(common.DnaBase, big.NewInt(9999999)),
				},
			},
			GodAddress:        godAddress,
			FirstCeremonyTime: node.CeremonyTime,
			GodAddressInvites: math.MaxUint16,
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
