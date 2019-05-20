package main

import (
	"gopkg.in/urfave/cli.v1"
	"idena-test-go/log"
	"idena-test-go/process"
	"os"
	"path/filepath"
)

func main() {
	app := cli.NewApp()
	app.Name = "idena-test"
	app.Version = "0.0.1"

	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "verbosity",
			Usage: "Log verbosity",
			Value: int(log.LvlInfo),
		},
		cli.IntFlag{
			Name:  "nodeverbosity",
			Usage: "Node log verbosity",
			Value: int(log.LvlTrace),
		},
		cli.IntFlag{
			Name:  "maxnetdelay",
			Usage: "Node max net delay",
			Value: 500,
		},
		cli.IntFlag{
			Name:  "users",
			Usage: "Users count",
			Value: 1,
		},
		cli.StringFlag{
			Name:  "workdir",
			Value: "workdir",
			Usage: "Workdir for nodes",
		},
		cli.StringFlag{
			Name:  "command",
			Value: "idena-go",
			Usage: "Command to run node",
		},
		cli.IntFlag{
			Name:  "ceremonyMinOffset",
			Usage: "First ceremony time offset in minutes",
			Value: int(log.LvlInfo),
		},
	}

	app.Action = func(context *cli.Context) error {

		workDir := context.String("workdir")

		createWorkDir(workDir)

		clearDataDir(filepath.Join(workDir, process.DataDir))

		logFileFullName := filepath.Join(workDir, process.DataDir, "bot.log")

		createLogFile(logFileFullName)

		fileHandler, err := log.FileHandler(logFileFullName, log.TerminalFormat(false))
		if err != nil {
			panic(err)
		}
		log.Root().SetHandler(log.LvlFilterHandler(log.Lvl(context.Int("verbosity")), log.MultiHandler(log.StreamHandler(os.Stdout, log.LogfmtFormat()), fileHandler)))

		process.NewProcess(
			context.Int("users"),
			workDir,
			context.String("command"),
			context.Int("nodeverbosity"),
			context.Int("maxnetdelay"),
			context.Int("ceremonyMinOffset"),
		).Start()
		return nil
	}

	app.Run(os.Args)
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
