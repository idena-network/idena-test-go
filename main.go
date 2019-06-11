package main

import (
	"fmt"
	"gopkg.in/urfave/cli.v1"
	"idena-test-go/api"
	"idena-test-go/config"
	"idena-test-go/initializer"
	"idena-test-go/log"
	"idena-test-go/process"
	"idena-test-go/scenario"
	"os"
	"path/filepath"
)

const godBotApiPort = 1111

func main() {
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
	}

	app.Action = func(context *cli.Context) error {

		if context.Bool("init") {
			initializer.InitMultiInsance(context.String("config"))
			return nil
		}

		conf := config.LoadFromFileWithDefaults(context.String("config"))

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
			conf.MaxNetDelay,
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
	if err := os.RemoveAll(dataDir); err != nil {
		panic(err)
	}
	if err := os.Mkdir(dataDir, os.ModePerm); err != nil {
		panic(err)
	}
}

func createLogFile(fullName string) {
	logFile, err := os.Create(fullName)
	if err != nil {
		panic(err)
	}
	logFile.Close()
}
