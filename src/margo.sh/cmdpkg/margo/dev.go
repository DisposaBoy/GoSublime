package margo

import (
	"fmt"
	"github.com/urfave/cli"
	"go/build"
	"margo.sh/cmdpkg/margo/cmdrunner"
)

const (
	devRemoteFork     = "origin"
	devRemoteUpstream = "margo"
	devUpstreamURL    = "https://margo.sh/"
)

var (
	devCmd = cli.Command{
		Name:        "dev",
		Description: "",
		Subcommands: cli.Commands{
			devCmdFork,
		},
	}

	devCmdFork = cli.Command{
		Name:        "fork",
		Description: "set remote `" + devRemoteFork + "` to your fork and `" + devRemoteUpstream + "` to the margo.sh repo",
		Action: func(cx *cli.Context) error {
			pkg, err := devCmdFindPkgDir()
			if err != nil {
				return err
			}

			args := cx.Args()
			if len(args) != 1 {
				return fmt.Errorf("Please specify the forked repo url")
			}

			cmds := []cmdrunner.Cmd{
				cmdrunner.Cmd{
					Name:     "git",
					Args:     []string{"remote", "add", "-f", devRemoteUpstream, devUpstreamURL},
					Dir:      pkg.Dir,
					OutToErr: true,
				},
				cmdrunner.Cmd{
					Name:     "git",
					Args:     []string{"remote", "set-url", "--push", devRemoteUpstream, "NoPushToMargoRepo"},
					Dir:      pkg.Dir,
					OutToErr: true,
				},
				cmdrunner.Cmd{
					Name:     "git",
					Args:     []string{"remote", "set-url", devRemoteFork, args[0]},
					Dir:      pkg.Dir,
					OutToErr: true,
				},
			}
			for _, cmd := range cmds {
				e := cmd.Run()
				if err == nil && e != nil {
					err = e
				}
			}
			return err
		},
	}
)

func devCmdFindPkgDir() (*build.Package, error) {
	pkg, err := build.Import("margo.sh", ".", build.FindOnly)
	if err == nil {
		return pkg, nil
	}

	err = cmdrunner.Cmd{
		Name:     "go",
		Args:     []string{"get", "-d", "-v", "margo.sh"},
		OutToErr: true,
	}.Run()
	if err != nil {
		return nil, fmt.Errorf("Cannot go get margo.sh: %s", err)
	}

	pkg, err = build.Import("margo.sh", ".", 0)
	if err != nil {
		return nil, fmt.Errorf("Cannot find pkg dir: %s", err)
	}
	return pkg, nil
}
