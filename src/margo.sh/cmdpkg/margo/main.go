package margo

import (
	"flag"
	"fmt"
	"github.com/urfave/cli"
	"margo.sh/mgcli"
	"margo.sh/sublime"
	"os"
)

var (
	cmdList = []mgcli.Commands{
		sublime.Commands,
	}

	cmdNames []string

	cmdMap map[string]mgcli.Commands

	buildCmd = cli.Command{
		Name:        "build",
		Description: "build the specified agent (see COMMANDS)",
	}

	runCmd = cli.Command{
		Name:        "run",
		Description: "run the specified agent (see COMMANDS)",
	}

	startCmd = cli.Command{
		Name:        "start",
		Description: "`build` and `run` the specified agent (see COMMANDS)",
	}
)

func init() {
	cmdMap = map[string]mgcli.Commands{}
	buildNames := []string{}
	runNames := []string{}
	for _, mc := range cmdList {
		cmdNames = append(cmdNames, mc.Name)
		cmdMap[mc.Name] = mc
		if mc.Build != nil {
			buildNames = append(buildNames, mc.Name)
			appendSubCmd(&buildCmd, mc, *mc.Build)
		}
		if mc.Run != nil {
			runNames = append(runNames, mc.Name)
			appendSubCmd(&runCmd, mc, *mc.Run)
		}
		appendSubCmd(&startCmd, mc, cli.Command{
			Action:          startAction,
			SkipFlagParsing: true,
			SkipArgReorder:  true,
		})
	}
}

func Main() {
	app := mgcli.NewApp()
	app.Commands = []cli.Command{
		buildCmd,
		runCmd,
		startCmd,
		devCmd,
		ciCmd,
	}
	app.RunAndExitOnError()
}

func appendSubCmd(cmd *cli.Command, cmds mgcli.Commands, subCmd cli.Command) {
	if subCmd.Name == "" {
		subCmd.Name = cmds.Name
	}
	cmd.Subcommands = append(cmd.Subcommands, subCmd)
}

func startAction(cx *cli.Context) error {
	mc := cmdMap[cx.Command.Name]
	app := &mgcli.NewApp().App
	app.Name = mc.Name
	app.ExitErrHandler = func(_ *cli.Context, _ error) {}
	newCtx := func(args []string) *cli.Context {
		flags := flag.NewFlagSet(mc.Name, 0)
		flags.Usage = func() {}
		flags.Parse(append([]string{mc.Name}, args...))
		return cli.NewContext(app, flags, cx)
	}
	if mc.Build != nil {
		err := mc.Build.Run(newCtx(nil))
		if err != nil {
			e := fmt.Sprintf("%s build failed: %s", mc.Name, err)
			os.Setenv("MARGO_BUILD_ERROR", e)
		}
	}
	if mc.Run != nil {
		return mc.Run.Run(newCtx(cx.Args()))
	}
	return nil
}
