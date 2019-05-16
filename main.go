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
		cli.IntFlag{
			Name:  "ceremonyMinOffset",
			Usage: "First ceremony time offset in minutes",
			Value: int(log.LvlInfo),
		},
	}

	app.Action = func(context *cli.Context) error {
		verbosity := context.Int("verbosity")
		logLvl := log.Lvl(verbosity)
		if runtime.GOOS == "windows" {
			log.Root().SetHandler(log.LvlFilterHandler(logLvl, log.StreamHandler(os.Stdout, log.LogfmtFormat())))
		} else {
			log.Root().SetHandler(log.LvlFilterHandler(logLvl, log.StreamHandler(os.Stderr, log.TerminalFormat(true))))
		}

		process.NewProcess(
			context.Int("users"),
			context.String("workdir"),
			context.String("command"),
			verbosity,
			context.Int("ceremonyMinOffset"),
		).Start()
		return nil
	}

	app.Run(os.Args)
}
