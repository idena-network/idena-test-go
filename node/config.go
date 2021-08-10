package node

import (
	"encoding/json"
	"github.com/idena-network/idena-go/common"
	"github.com/idena-network/idena-go/config"
	"github.com/imdario/mergo"
	"math"
	"math/big"
	"path/filepath"
	"time"
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
	MaxInboundPeers  *int `json:"MaxInboundPeers,omitempty"`
	MaxOutboundPeers *int `json:"MaxOutboundPeers,omitempty"`
	MaxDelay         *int
	CollectMetrics   bool
	Multishard       bool
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
	defaultConfig, err := config.MakeConfigFromFile("")
	if err != nil {
		panic(err)
	}
	conf := node.buildSpecificConfig()
	if node.baseConfigData == nil {
		return conf
	}
	return addConfig(defaultConfig, conf, node.baseConfigData)
}

func (node *Node) buildNodeConfigFileData() []byte {
	c := buildConfig(node)

	bytes, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}

	return bytes
}

func addConfig(defaultConfig *config.Config, c *specConfig, configData []byte) interface{} {
	//defaultConfigData, err := json.Marshal(defaultConfig)
	//if err != nil {
	//	panic(err)
	//}
	//defaultConfigMap := make(map[string]interface{})
	//if err := json.Unmarshal(defaultConfigData, &defaultConfigMap); err != nil {
	//	panic(err)
	//}
	resMap := make(map[string]interface{})
	//if err := mergo.Merge(&resMap, defaultConfigMap); err != nil {
	//	panic(err)
	//}
	cData, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	cMap := make(map[string]interface{})
	if err := json.Unmarshal(cData, &cMap); err != nil {
		panic(err)
	}
	if err := mergo.Merge(&resMap, cMap, mergo.WithOverride); err != nil {
		panic(err)
	}
	if configData != nil {
		configDataMap := make(map[string]interface{})
		if err := json.Unmarshal(configData, &configDataMap); err != nil {
			panic(err)
		}
		if err := mergo.Merge(&resMap, configDataMap, mergo.WithOverride); err != nil {
			panic(err)
		}
	}
	resMap["GenesisConf"].(map[string]interface{})["Alloc"] = c.GenesisConf.Alloc
	return resMap
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
	allFlipsLoadingTime := time.Hour * 2
	if node.isShared {
		allFlipsLoadingTime = time.Minute * 4
	}
	return &specConfig{
		Network: &nw,
		DataDir: dataDir,
		P2P: p2pConfig{
			MaxDelay:   &maxNetDelay,
			Multishard: node.isShared,
		},
		RPC: rpcConfig{
			HTTPHost: node.RpcHost,
			HTTPPort: node.RpcPort,
		},
		GenesisConf: genesisConf{
			Alloc: map[string]genesisAllocation{
				godAddress: {
					Balance: big.NewInt(0).Mul(common.DnaBase, big.NewInt(99999999)),
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
		Sync: &config.SyncConfig{
			FastSync:            true,
			ForceFullSync:       config.DefaultForceFullSync,
			LoadAllFlips:        node.isShared,
			AllFlipsLoadingTime: allFlipsLoadingTime,
		},
	}
}
