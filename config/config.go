package config

import (
	"encoding/json"
	"github.com/idena-network/idena-test-go/common"
	"github.com/idena-network/idena-test-go/log"
	"io/ioutil"
	"os"
)

type Config struct {
	Verbosity           int
	NodeVerbosity       int
	MaxNetDelay         *int
	WorkDir             string
	Command             string
	Scenario            string
	NodeConfig          string
	RpcAddr             string
	GodMode             bool
	GodHost             string
	PortOffset          int
	NodeStartWaitingSec int
	NodeStopWaitingSec  int
}

func LoadFromFileWithDefaults(path string, godBotMode bool, portOffset int) Config {
	configResult := defaultConfig()
	if len(path) == 0 {
		return configResult
	}
	var configJson []byte
	var err error
	if common.IsValidUrl(path) {
		configJson, err = common.LoadData(path)
	} else {
		configJson, err = ioutil.ReadFile(path)
	}
	if err != nil {
		panic(err)
	}
	c := Config{}
	if err := json.Unmarshal(configJson, &c); err != nil {
		panic(err)
	}
	merge(&c, &configResult)
	if godBotMode {
		configResult.GodMode = godBotMode
	}
	if portOffset > 0 {
		configResult.PortOffset = portOffset
	}
	return configResult
}

func defaultConfig() Config {
	defaultWorkDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	defaultMaxNetDelay := 500
	return Config{
		Verbosity:           int(log.LvlInfo),
		NodeVerbosity:       int(log.LvlTrace),
		MaxNetDelay:         &defaultMaxNetDelay,
		WorkDir:             defaultWorkDir,
		Command:             "idena-go",
		RpcAddr:             "localhost",
		NodeStartWaitingSec: 10,
		NodeStopWaitingSec:  4,
	}
}

func merge(from *Config, to *Config) {
	if from.Verbosity > 0 {
		to.Verbosity = from.Verbosity
	}
	if from.NodeVerbosity > 0 {
		to.NodeVerbosity = from.NodeVerbosity
	}
	if from.MaxNetDelay != nil {
		to.MaxNetDelay = from.MaxNetDelay
	}
	if len(from.WorkDir) > 0 {
		to.WorkDir = from.WorkDir
	}
	if len(from.Command) > 0 {
		to.Command = from.Command
	}
	if len(from.RpcAddr) > 0 {
		to.RpcAddr = from.RpcAddr
	}
	to.Scenario = from.Scenario
	to.NodeConfig = from.NodeConfig
	to.GodMode = from.GodMode
	to.GodHost = from.GodHost
	to.PortOffset = from.PortOffset
	to.NodeStartWaitingSec = from.NodeStartWaitingSec
	to.NodeStopWaitingSec = from.NodeStopWaitingSec
}
