package margo

import (
	"disposa.blue/margo/mgcli"
	"disposa.blue/margo/sublime"
	"github.com/urfave/cli"
)

func Main() {
	app := mgcli.NewApp()
	app.Commands = []cli.Command{
		sublime.Command,
	}
	app.RunAndExitOnError()
}
