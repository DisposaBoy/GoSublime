package mg

import (
	"go/build"
	"os/exec"
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

func (r restartSupport) Reduce(mx *Ctx) *State {
	switch mx.Action.(type) {
	case ViewSaved:
		return r.viewSaved(mx)
	case Restart:
		mx.Log.Printf("%T action dispatched\n", mx.Action)
		return mx.addClientActions(clientRestart)
	case Shutdown:
		mx.Log.Printf("%T action dispatched\n", mx.Action)
		return mx.addClientActions(clientShutdown)
	default:
	}
	return mx.State
}

func (r restartSupport) viewSaved(mx *Ctx) *State {
	go r.prepRestart(mx)
	return mx.State
}

func (_ restartSupport) prepRestart(mx *Ctx) {
	dir := filepath.ToSlash(mx.View.Dir())
	if !filepath.IsAbs(dir) {
		return
	}

	// if we use build..ImportPath, it will be wrong if we work on the code outside the GS GOPATH
	imp := ""
	if i := strings.LastIndex(dir, "/src/"); i >= 0 {
		imp = dir[i+5:]
	}
	if imp != "margo" && !strings.HasPrefix(imp+"/", "disposa.blue/margo/") {
		return
	}

	pkg, _ := build.Default.ImportDir(dir, 0)
	if pkg == nil || pkg.Name == "" {
		return
	}

	defer mx.Begin(Task{Title: "prepping margo restart"}).Done()

	cmd := exec.Command("go", "test")
	cmd.Dir = mx.View.Dir()
	cmd.Env = mx.Env.Environ()
	out, err := cmd.CombinedOutput()
	msg := "telling margo to restart after " + mx.View.Filename() + " was saved"
	if err == nil {
		mx.Log.Println(msg)
		mx.Store.Dispatch(Restart{})
	} else {
		mx.Log.Printf("not %s: go test failed: %s\n%s\n", msg, err, out)
	}
}
