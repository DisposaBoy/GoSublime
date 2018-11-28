package margo

import (
	"github.com/urfave/cli"
	"margo.sh/cmdpkg/margo/cmdrunner"
	"strings"
)

var ciCmd = cli.Command{
	Name:        "ci",
	Description: "ci runs various tests for use in ci environments, etc.",
	ArgsUsage:   "[patterns...] (default 'margo.sh/...')",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "quick",
			Usage: "Disable '-race' and other things that are known to be slow.",
		},
	},
	Action: func(cx *cli.Context) error {
		quick := cx.Bool("quick")
		race := !quick
		pats := cx.Args()
		if len(pats) == 0 {
			pats = []string{"margo.sh/..."}
		}

		testScript := []string{"go", "test"}
		if race {
			testScript = append(testScript, "-race")
		}

		vetScript := []string{"go", "vet",
			"-all",
			"-printfuncs", strings.Join([]string{
				"Errorf",
				"Fatal", "Fatalf",
				"Fprint", "Fprintf", "Fprintln",
				"Panic", "Panicf", "Panicln",
				"Print", "Printf", "Println",
				"Sprint", "Sprintf", "Sprintln",

				"AddErrorf",
				"AddStatusf",
				"dbgf",
				"EmTextf",
				"Textf",
			}, ","),
		}

		scripts := [][]string{
			vetScript,
			testScript,
		}
		for _, script := range scripts {
			cmd := cmdrunner.Cmd{
				Name:     script[0],
				Args:     append(script[1:], pats...),
				OutToErr: true,
			}
			if err := cmd.Run(); err != nil {
				return err
			}
		}
		return nil
	},
}
