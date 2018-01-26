package mgcli

import (
	"fmt"
	"github.com/urfave/cli"
	"os"
)

type App struct{ cli.App }

func (a *App) RunAndExitOnError() {
	if err := a.Run(os.Args); err != nil {
		fmt.Fprintln(a.ErrWriter, "error:", err)
		os.Exit(1)
	}
}

func Action(f cli.ActionFunc) cli.ActionFunc {
	return func(c *cli.Context) error {
		return Error("", f(c))
	}
}

func NewApp() *App {
	a := cli.NewApp()
	a.Usage = ""
	a.Version = ""
	a.Writer = os.Stderr
	a.ErrWriter = os.Stderr
	return &App{App: *a}
}

func Error(message string, err error) error {
	switch {
	case err == nil:
		return nil
	case message == "":
		return cli.NewExitError(err, 1)
	default:
		return cli.NewExitError(fmt.Sprintf("%s: %s", message, err), 1)
	}
}
