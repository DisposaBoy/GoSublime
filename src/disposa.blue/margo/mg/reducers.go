package mg

import (
	"go/build"
	"path/filepath"
	"strings"
)

var (
	DefaultReducers = []Reducer{
		RestartOnSave{},
	}
)

type RestartOnSave struct{}

func (_ RestartOnSave) Reduce(mx *Ctx) *State {
	if _, ok := mx.Action.(ViewSaved); !ok {
		return mx.State
	}

	dir := filepath.ToSlash(filepath.Dir(mx.View.Path))
	if dir == "" {
		return mx.State
	}

	// if we use build..ImportPath, it will be wrong if we work on the code outside the GS GOPATH
	imp := ""
	if i := strings.LastIndex(dir, "/src/"); i >= 0 {
		imp = dir[i+5:]
	}
	if imp != "margo" && !strings.HasPrefix(imp+"/", "disposa.blue/margo/") {
		return mx.State
	}

	pkg, _ := build.Default.ImportDir(dir, 0)
	if pkg == nil || pkg.Name == "" {
		return mx.State
	}

	return mx.MarkObsolete()
}
