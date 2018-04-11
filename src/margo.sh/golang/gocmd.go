package golang

import (
	"fmt"
	"margo.sh/mg"
	"strings"
)

type GoCmd struct{}

func (gc *GoCmd) Reduce(mx *mg.Ctx) *mg.State {
	switch act := mx.Action.(type) {
	case mg.RunCmd:
		return gc.runCmd(mx, act)
	}
	return mx.State
}

func (gc *GoCmd) runCmd(mx *mg.Ctx, rc mg.RunCmd) *mg.State {
	return mx.State.AddBuiltinCmds(
		mg.BultinCmd{Run: gc.gotoolCmd, Name: "go", Desc: "" +
			"Wrapper around the go command." +
			" It adds `go .play` and `go .replay` as well as add linter support." +
			"",
		},
	)
}

func (gc *GoCmd) subCmd(args []string) string {
	for _, s := range args {
		if s != "" && !strings.HasPrefix(s, "-") {
			return s
		}
	}
	return ""
}

func (gc *GoCmd) gotoolCmd(bx *mg.BultinCmdCtx) *mg.State {
	go gc.gotool(bx)
	return bx.State
}

func (gc *GoCmd) gotool(bx *mg.BultinCmdCtx) {
	defer bx.Close()

	subCmd := gc.subCmd(bx.Args)
	dir := bx.View.Dir()
	// TODO: detect pkgDir passed os args
	pkgDir := dir
	type Key struct{ subCmd, pkgDir string }
	key := Key{subCmd, pkgDir}

	iw := &mg.IssueWriter{
		Patterns: CommonPatterns,
		Base:     mg.Issue{Label: strings.TrimSpace("go " + subCmd)},
		Dir:      dir,
	}

	bx = bx.Copy(func(bx *mg.BultinCmdCtx) {
		bx.Output = &mg.CmdOutputWriter{
			Fd:     bx.Output.Fd,
			Writer: iw,
		}
	})

	err := bx.RunProc()
	if err != nil {
		fmt.Fprintln(bx.Output, err)
		return
	}
	iw.Flush()

	bx.Store.Dispatch(mg.StoreIssues{
		Key:    mg.IssueKey{Key: key, Dir: pkgDir},
		Issues: iw.Issues(),
	})
}
