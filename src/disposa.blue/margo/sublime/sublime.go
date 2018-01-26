package sublime

import (
	"disposa.blue/margo/mgcli"
	"fmt"
	"github.com/urfave/cli"
	"go/build"
	"os"
	"os/exec"
	"strings"
)

var (
	Command = cli.Command{
		Name:            "sublime",
		Aliases:         []string{"subl"},
		Usage:           "",
		Description:     "",
		SkipFlagParsing: true,
		SkipArgReorder:  true,
		Action:          mgcli.Action(mainAction),
	}
)

type cmdHelper struct {
	name     string
	args     []string
	outToErr bool
	env      []string
}

func (c cmdHelper) run() error {
	cmd := exec.Command(c.name, c.args...)
	cmd.Stdin = os.Stdin
	if c.outToErr {
		cmd.Stdout = os.Stderr
	} else {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = os.Stderr
	cmd.Env = c.env

	err := cmd.Run()
	status := "ok"
	if err != nil {
		status = "error: " + err.Error()
	}
	fmt.Fprintf(os.Stderr, "run%q: %s\n", append([]string{c.name}, c.args...), status)
	return err
}

func mainAction(c *cli.Context) error {
	args := c.Args()
	tags := []string{"margo"}
	if extensionPkgExists() {
		tags = []string{"margo margo_extension", "margo"}
	}
	if err := goInstallAgent(os.Getenv("MARGO_SUBLIME_GOPATH"), tags); err != nil {
		return fmt.Errorf("cannot install margo.sublime: %s", err)
	}
	return cmdHelper{name: "margo.sublime", args: args}.run()
}

func goInstallAgent(gp string, tags []string) error {
	var env []string
	if gp != "" {
		env = make([]string, 0, len(os.Environ())+1)
		// I don't remember the rules about duplicate env vars...
		for _, s := range os.Environ() {
			if !strings.HasPrefix(s, "GOPATH=") {
				env = append(env, s)
			}
		}
		env = append(env, "GOPATH="+gp)
	}

	cmdpath := "disposa.blue/margo/cmd/margo.sublime"
	if s := os.Getenv("MARGO_SUBLIME_CMDPATH"); s != "" {
		cmdpath = s
	}

	var err error
	for _, tag := range tags {
		err = cmdHelper{
			name:     "go",
			args:     []string{"install", "-v", "-tags", tag, cmdpath},
			outToErr: true,
			env:      env,
		}.run()
		if err == nil {
			return nil
		}
	}
	return err
}

func extensionPkgExists() bool {
	_, err := build.Import("margo", "", 0)
	return err == nil
}
