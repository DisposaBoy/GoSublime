package mg

import (
	"go/build"
	"margo.sh/mgutil"
	"os"
	"os/exec"
	"strings"
)

var (
	// we need to use the env that we started with, not the user's env
	buildCtx = build.Default
)

type rsIssues struct {
	ActionType
	issues IssueSet
}

type restartSupport struct {
	ReducerType
	q      *mgutil.ChanQ
	issues IssueSet
}

func (rs *restartSupport) ReducerLabel() string {
	return "Mg/Restart"
}

func (rs *restartSupport) ReducerInit(mx *Ctx) {
	go rs.onInit(mx)
}

func (rs *restartSupport) ReducerCond(mx *Ctx) bool {
	if len(rs.issues) != 0 || mx.ActionIs(rsIssues{}) {
		return true
	}
	if mx.LangIs(Go) && mx.ActionIs(ViewSaved{}) {
		return true
	}
	return false
}

func (rs *restartSupport) ReducerMount(mx *Ctx) {
	rs.q = mgutil.NewChanQ(1)
	go rs.loop()
}

func (rs *restartSupport) ReducerUnmount(mx *Ctx) {
	rs.q.Close()
}

func (rs *restartSupport) Reduce(mx *Ctx) *State {
	switch act := mx.Action.(type) {
	case rsIssues:
		rs.issues = act.issues
	case ViewSaved:
		rs.q.Put(mx)
	}
	return mx.State.AddIssues(rs.issues...)
}

func (rs *restartSupport) loop() {
	for v := range rs.q.C() {
		rs.onSave(v.(*Ctx))
	}
}

func (rs *restartSupport) mgPkg(mx *Ctx) *build.Package {
	v := mx.View
	if !strings.HasSuffix(v.Path, ".go") {
		return nil
	}

	pkg, _ := buildCtx.ImportDir(mx.View.Dir(), 0)
	if pkg == nil || pkg.ImportPath == "" {
		return nil
	}
	imp := pkg.ImportPath + "/"
	if !strings.HasPrefix(imp, "margo/") && !strings.HasPrefix(imp, "margo.sh/") {
		return nil
	}
	return pkg
}

func (rs *restartSupport) onInit(mx *Ctx) {
	if os.Getenv("MARGO_BUILD_ERROR") == "" {
		return
	}

	res := rsIssues{issues: rs.slowLint(mx)}
	if len(res.issues) != 0 {
		mx.Store.Dispatch(res)
	}
}

func (rs *restartSupport) onSave(mx *Ctx) {
	pkg := rs.mgPkg(mx)
	if pkg == nil {
		return
	}

	res := rsIssues{issues: rs.slowLint(mx)}
	mx.Store.Dispatch(res)
	if len(res.issues) == 0 {
		mx.Log.Println(pkg.ImportPath, "saved with no issues, restarting")
		mx.Store.Dispatch(Restart{})
	}
}

func (rs *restartSupport) slowLint(mx *Ctx) IssueSet {
	defer mx.Begin(Task{Title: "prepping margo restart"}).Done()

	cmd := exec.Command("margo.sh", "build", mx.AgentName())
	cmd.Dir = mx.View.Dir()
	cmd.Env = mx.Env.Environ()
	out, err := cmd.CombinedOutput()

	iw := &IssueWriter{
		Dir:      mx.View.Dir(),
		Patterns: mx.CommonPatterns(),
		Base:     Issue{Label: rs.ReducerLabel()},
	}
	iw.Write(out)
	iw.Close()
	issues := iw.Issues()

	if err != nil {
		mx.Log.Printf(rs.ReducerLabel()+": %s\n%s\n", err, out)
	}
	return issues
}
