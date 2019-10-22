package main

import (
	"fmt"
	"github.com/idena-network/idena-test-go/api"
	"github.com/idena-network/idena-test-go/config"
	"github.com/idena-network/idena-test-go/initializer"
	"github.com/idena-network/idena-test-go/log"
	"github.com/idena-network/idena-test-go/process"
	"github.com/idena-network/idena-test-go/scenario"
	"gopkg.in/urfave/cli.v1"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"time"
)

const godBotApiPort = 1111

func main() {
	rand.Seed(time.Now().UnixNano())
	app := cli.NewApp()
	app.Name = "idena-test"
	app.Version = "0.0.1"

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "init",
			Usage: "Init multi instance",
		},
		cli.StringFlag{
			Name:  "config",
			Usage: "Config file",
			Value: "config.json",
		},
		cli.StringFlag{
			Name:  "godBotHost",
			Usage: "God bot host",
			Value: "",
		},
		cli.BoolFlag{
			Name:  "godBotMode",
			Usage: "God bot mode",
		},
	}

	app.Action = func(context *cli.Context) error {

		if context.Bool("init") {
			initializer.InitMultiInsance(context.String("config"))
			return nil
		}

		conf := config.LoadFromFileWithDefaults(context.String("config"), context.String("godBotHost"),
			context.Bool("godBotMode"))

		workDir := conf.WorkDir
		initApp(workDir, conf.Verbosity)

		scenarioFileName := conf.Scenario

		var sc scenario.Scenario
		if len(scenarioFileName) > 0 {
			sc = scenario.Load(filepath.Join(workDir, scenarioFileName))
		} else {
			sc = scenario.GetDefaultScenario()
		}

		p := process.NewProcess(
			sc,
			conf.PortOffset,
			workDir,
			conf.Command,
			conf.NodeConfig,
			conf.RpcAddr,
			conf.NodeVerbosity,
			*conf.MaxNetDelay,
			conf.GodMode,
			conf.GodHost,
		)

		if conf.GodMode {
			apiPort := godBotApiPort
			go api.NewApi(p, apiPort)
			log.Info(fmt.Sprintf("Run http API server, port %d", apiPort))
		}

		p.Start()
		return nil
	}

	app.Run(os.Args)
}

func initApp(workDir string, verbosity int) {

	createWorkDir(workDir)

	clearDataDir(filepath.Join(workDir, process.DataDir))

	logFileFullName := filepath.Join(workDir, process.DataDir, "bot.log")

	createLogFile(logFileFullName)

	fileHandler, err := log.FileHandler(logFileFullName, log.TerminalFormat(false))
	if err != nil {
		panic(err)
	}
	log.Root().SetHandler(log.LvlFilterHandler(log.Lvl(verbosity), log.MultiHandler(log.StreamHandler(os.Stdout, log.LogfmtFormat()), fileHandler)))
}

func createWorkDir(workDir string) {
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		err := os.MkdirAll(workDir, os.ModePerm)
		if err != nil {
			panic(err)
		}
	}
}

func clearDataDir(dataDir string) {
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		if err := os.Mkdir(dataDir, os.ModePerm); err != nil {
			panic(err)
		}
		return
	}
	dirRead, err := os.Open(dataDir)
	if err != nil {
		panic(err)
	}
	dirFiles, err := dirRead.Readdir(0)
	if err != nil {
		panic(err)
	}

	for index := range dirFiles {
		filename := dirFiles[index].Name()
		fullPath := path.Join(dataDir, filename)
		err := os.RemoveAll(fullPath)
		if err != nil {
			panic(err)
		}
	}
}

func createLogFile(fullName string) {
	logFile, err := os.Create(fullName)
	if err != nil {
		panic(err)
	}
	logFile.Close()
}
