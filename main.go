package main

import (
	"gopkg.in/urfave/cli.v1"
	"idena-test-go/log"
	"idena-test-go/process"
	"os"
	"runtime"
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
	}

	app.Action = func(context *cli.Context) error {
		logLvl := log.Lvl(context.Int("verbosity"))
		if runtime.GOOS == "windows" {
			log.Root().SetHandler(log.LvlFilterHandler(logLvl, log.StreamHandler(os.Stdout, log.LogfmtFormat())))
		} else {
			log.Root().SetHandler(log.LvlFilterHandler(logLvl, log.StreamHandler(os.Stderr, log.TerminalFormat(true))))
		}

		process.NewProcess(
			context.Int("users"),
			context.String("workdir"),
			context.String("command"),
		).Start()
		return nil
	}

	app.Run(os.Args)
}
