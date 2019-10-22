package initializer

import (
	"bufio"
	"encoding/json"
	"fmt"
	botconfig "github.com/idena-network/idena-test-go/config"
	"io"
	"os"
	"path/filepath"
)

const outDir = "out"

type initializer struct {
	conf config
}

func InitMultiInsance(configFile string) {

	in := &initializer{}

	deleteOutDir()

	createOutDir()

	in.conf = loadFromFile(configFile)

	for i := 0; i < in.conf.Instances; i++ {
		in.initInstance(i)
	}
}

func deleteOutDir() {
	if _, err := os.Stat(outDir); os.IsNotExist(err) {
		return
	}
	if err := os.RemoveAll(outDir); err != nil {
		panic(err)
	}
}

func createOutDir() {
	if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
		panic(err)
	}
}

func (in *initializer) initInstance(index int) {
	workDir := fmt.Sprintf("botWorkDir-%d", index)
	err := os.MkdirAll(filepath.Join(outDir, workDir), os.ModePerm)
	if err != nil {
		panic(err)
	}

	copyFile(in.conf.BotApp, filepath.Join(outDir, workDir, in.conf.BotApp))
	copyFile(in.conf.NodeApp, filepath.Join(outDir, workDir, in.conf.NodeApp))

	if len(in.conf.Scenario) > 0 {
		copyFile(in.conf.Scenario, filepath.Join(outDir, workDir, "scenario.json"))
	}

	if len(in.conf.NodeBaseConfig) > 0 {
		copyFile(in.conf.NodeBaseConfig, filepath.Join(outDir, workDir, "nodeConfig.json"))
	}

	botConfig := botconfig.LoadFromFileWithDefaults(in.conf.BotConfig, "", false)
	if index == 0 {
		botConfig.GodMode = true
	} else {
		botConfig.PortOffset = in.conf.PortOffset * index
	}
	botConfig.GodHost = in.conf.GodHost

	if len(in.conf.Scenario) > 0 {
		botConfig.Scenario = "scenario.json"
	}

	if len(in.conf.NodeBaseConfig) > 0 {
		botConfig.NodeConfig = "nodeConfig.json"
	}

	botConfig.WorkDir = ""

	bytes, err := json.Marshal(botConfig)
	if err != nil {
		panic(err)
	}
	writeToFile(bytes, filepath.Join(outDir, workDir, "config.json"))
}

func copyFile(src, dst string) {
	source, err := os.Open(src)
	if err != nil {
		panic(err)
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		panic(err)
	}
	defer destination.Close()
	_, err = io.Copy(destination, source)
	if err != nil {
		panic(err)
	}
}

func writeToFile(data []byte, filePath string) {
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	_, err = w.Write(data)
	if err != nil {
		panic(err)
	}
	w.Flush()
}
