package mg

import (
	"go/build"
	"path/filepath"
	"strings"
)

var (
	DefaultReducers = []Reducer{
		RestartOnSave,
	}
)

func RestartOnSave(st State, act Action) State {
	switch act.(type) {
	case ViewSaved:
		dir := filepath.Dir(st.View.Path)
		if dir == "" {
			break
		}

		// if we use build..ImportPath, it will be wrong if we work on the code outside the GS GOPATH
		imp := ""
		if i := strings.LastIndex(dir, "/src/"); i >= 0 {
			imp = dir[i+5:]
		}
		if imp != "margo" && !strings.HasPrefix(imp+"/", "disposa.blue/margo/") {
			break
		}

		pkg, _ := build.Default.ImportDir(dir, 0)
		if pkg == nil || pkg.Name == "" {
			break
		}

		st.Obsolete = true
	}
	return st
}
