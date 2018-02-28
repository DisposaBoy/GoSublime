package mg

import (
	"go/build"
	"path/filepath"
	"strings"
)

var (
	defaultReducers = struct {
		before, use, after []Reducer
	}{
		before: []Reducer{
			restartSupport{},
		},
		after: []Reducer{
			issueSupport{},
		},
	}
)

type restartSupport struct{}

func (_ restartSupport) Reduce(mx *Ctx) *State {
	if _, ok := mx.Action.(ViewSaved); !ok {
		return mx.State
	}

	dir := filepath.ToSlash(mx.View.Dir())
	if !filepath.IsAbs(dir) {
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
