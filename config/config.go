package config

import (
	"encoding/json"
	"idena-test-go/log"
	"io/ioutil"
	"os"
)

type Config struct {
	Verbosity     int
	NodeVerbosity int
	MaxNetDelay   int
	WorkDir       string
	Command       string
	Scenario      string
	NodeConfig    string
	RpcAddr       string
	GodMode       bool
	GodHost       string
	PortOffset    int
}

func LoadFromFileWithDefaults(path string) Config {
	configResult := defaultConfig()
	if len(path) == 0 {
		return configResult
	}
	configJson, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	c := Config{}
	if err := json.Unmarshal(configJson, &c); err != nil {
		panic(err)
	}
	merge(&c, &configResult)
	return configResult
}

func defaultConfig() Config {
	defaultWorkDir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return Config{
		Verbosity:     int(log.LvlInfo),
		NodeVerbosity: int(log.LvlTrace),
		MaxNetDelay:   500,
		WorkDir:       defaultWorkDir,
		Command:       "idena-go",
		RpcAddr:       "localhost",
	}
}

func merge(from *Config, to *Config) {
	if from.Verbosity > 0 {
		to.Verbosity = from.Verbosity
	}
	if from.NodeVerbosity > 0 {
		to.NodeVerbosity = from.NodeVerbosity
	}
	if from.MaxNetDelay > 0 {
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
}
