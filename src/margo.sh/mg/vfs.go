package mg

import (
	"margo.sh/vfs"
	"path/filepath"
)

var (
	vFS = vfs.New()
)

type vfsCmd struct{ ReducerType }

func (vc *vfsCmd) Reduce(mx *Ctx) *State {
	switch mx.Action.(type) {
	case ViewSaved:
		go vc.sync(mx)
	case RunCmd:
		return mx.AddBuiltinCmds(BuiltinCmd{
			Name: ".vfs",
			Desc: "Print a tree representing the default VFS",
			Run:  vc.run,
		})
	}
	return mx.State
}

func (vc *vfsCmd) run(cx *CmdCtx) *State {
	go vc.cmd(cx)
	return cx.State
}

func (vc *vfsCmd) cmd(cx *CmdCtx) {
	defer cx.Output.Close()

	if len(cx.Args) == 0 {
		vFS.Print(cx.Output)
		return
	}

	for _, p := range cx.Args {
		nd, pat := &vFS.Node, p
		if filepath.IsAbs(p) {
			nd, pat = vFS.Peek(filepath.Dir(p)), filepath.Base(p)
		}
		nd.PrintWithFilter(cx.Output, func(nd *vfs.Node) string {
			if nd.IsBranch() {
				return nd.String()
			}
			if ok, _ := filepath.Match(pat, nd.Name()); ok {
				return nd.String()
			}
			return ""
		})
	}
}

func (vc *vfsCmd) sync(mx *Ctx) {
	v := mx.View
	vFS.Invalidate(v.Filename())
	vFS.Invalidate(v.Dir())
}

func init() {
	DefaultReducers.Before(&vfsCmd{})
}
