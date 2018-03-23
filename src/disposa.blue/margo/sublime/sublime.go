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

	fmt.Fprintf(os.Stderr, "run%q\n", append([]string{c.name}, c.args...))
	return cmd.Run()
}

func mainAction(c *cli.Context) error {
	args := c.Args()
	tags := "margo"
	if extensionPkgExists() {
		tags = "margo margo_extension"
	}
	var env []string
	if err := goInstallAgent(os.Getenv("MARGO_SUBLIME_GOPATH"), tags); err != nil {
		env = append(env, "MARGO_SUBLIME_INSTALL_FAILED=margo install failed. check console for errors")
		fmt.Fprintln(os.Stderr, "cannot install margo.sublime:", err)
	}
	name := "margo.sublime"
	if exe, err := exec.LookPath(name); err == nil {
		name = exe
	}
	return cmdHelper{name: name, args: args, env: env}.run()
}

func goInstallAgent(gp string, tags string) error {
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

	args := []string{"install", "-v", "-tags=" + tags}
	if os.Getenv("MARGO_INSTALL_FLAGS_RACE") == "1" {
		args = append(args, "-race")
	}
	for _, tag := range build.Default.ReleaseTags {
		if tag == "go1.10" {
			args = append(args, "-i")
			break
		}
	}
	args = append(args, cmdpath)
	return cmdHelper{
		name:     "go",
		args:     args,
		outToErr: true,
		env:      env,
	}.run()
}

func extensionPkgExists() bool {
	_, err := build.Import("margo", "", 0)
	return err == nil
}
