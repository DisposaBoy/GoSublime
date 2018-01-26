package margosublime

import (
	"disposa.blue/margo/mg"
	"disposa.blue/margo/mgcli"
	"fmt"
	"github.com/urfave/cli"
)

var initFuncs []func(*mg.Store)

func Main() {
	cfg := mg.AgentConfig{}
	app := mgcli.NewApp()
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "codec",
			Value:       cfg.Codec,
			Destination: &cfg.Codec,
			Usage:       fmt.Sprintf("The IPC codec: %s (default %s)", mg.CodecNamesStr, mg.DefaultCodec),
		},
	}
	app.Action = func(ctx *cli.Context) error {
		if ctx.Args().Present() {
			return cli.ShowAppHelp(ctx)
		}

		ag, err := mg.NewAgent(cfg)
		if err != nil {
			return mgcli.Error("agent creation failed:", err)
		}
		for _, cf := range initFuncs {
			cf(ag.Store)
		}
		if err := ag.Run(); err != nil {
			return mgcli.Error("agent failed:", err)
		}
		return nil
	}
	app.RunAndExitOnError()
}
