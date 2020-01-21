package mg

import (
	"fmt"
	hmnz "github.com/dustin/go-humanize"
	"margo.sh/vfs"
	"path/filepath"
	"strings"
)

var (
	vFS = vfs.New()
)

type vfsCmd struct{ ReducerType }

func (vc *vfsCmd) Reduce(mx *Ctx) *State {
	v := mx.View
	switch mx.Action.(type) {
	case ViewModified, ViewLoaded:
		mx.VFS.Invalidate(v.Name)
	case ViewSaved:
		mx.VFS.Invalidate(v.Name)
		mx.VFS.Invalidate(v.Filename())
	case RunCmd:
		return mx.AddBuiltinCmds(
			BuiltinCmd{
				Name: ".vfs",
				Desc: "Print a tree representing the default VFS",
				Run: func(cx *CmdCtx) *State {
					go vc.cmdVfs(cx)
					return cx.State
				},
			},
			BuiltinCmd{
				Name: ".vfs-blobs",
				Desc: "Print a list, and summary of, blobs (file contents) cached in the VFS.",
				Run: func(cx *CmdCtx) *State {
					go vc.cmdVfsBlobs(cx)
					return cx.State
				},
			},
		)
	}
	return mx.State
}

func (vc *vfsCmd) cmdVfs(cx *CmdCtx) {
	defer cx.Output.Close()

	if len(cx.Args) == 0 {
		cx.VFS.Print(cx.Output)
		return
	}

	for _, p := range cx.Args {
		nd, pat := &cx.VFS.Node, p
		if filepath.IsAbs(p) {
			nd, pat = cx.VFS.Peek(filepath.Dir(p)), filepath.Base(p)
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

func (vc *vfsCmd) cmdVfsBlobs(cx *CmdCtx) {
	defer cx.Output.Close()

	files := int64(0)
	size := uint64(0)
	cx.VFS.PrintWithFilter(cx.Output, func(nd *vfs.Node) string {
		if nd.IsBranch() {
			return nd.String()
		}
		nm := []string{}
		for _, b := range vfs.Blobs(nd) {
			files++
			sz := uint64(b.Len())
			size += sz
			nm = append(nm, fmt.Sprintf("%s (%s)", nd.String(), hmnz.IBytes(sz)))
		}
		return strings.Join(nm, ", ")
	})
	fmt.Fprintf(cx.Output, "\n%s files (%s) cached in memory.",
		hmnz.Comma(files), hmnz.IBytes(size),
	)
}

func init() {
	DefaultReducers.Before(&vfsCmd{})
}
