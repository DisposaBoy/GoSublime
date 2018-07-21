package sublime

import (
	"margo.sh/mg"
	"runtime"
)

func Margo(ma mg.Args) {
	ma.Store.Use(mg.NewReducer(func(mx *mg.Ctx) *mg.State {
		ctrl := "ctrl"
		if runtime.GOOS == "darwin" {
			ctrl = "super"
		}
		return mx.AddStatusf("press ` %s+. `,` %s+x ` to configure margo", ctrl, ctrl)
	}))
}
