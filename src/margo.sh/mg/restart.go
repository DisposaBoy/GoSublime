package mg

import (
	"bytes"
	"go/build"
	"margo.sh/mgutil"
	"os"
	"os/exec"
	"strings"
)

var (
	// we need to use the env that was used to build the agent, not the user's env
	agentBuildCtx = func() build.Context {
		c := build.Default
		if s := os.Getenv("MARGO_AGENT_GOPATH"); s != "" {
			c.GOPATH = s
		}
		return c
	}()
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

	pkg, _ := agentBuildCtx.ImportDir(mx.View.Dir(), 0)
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

	res := rsIssues{issues: rs.slowLint(mx, nil)}
	if len(res.issues) != 0 {
		mx.Store.Dispatch(res)
	}
}

func (rs *restartSupport) onSave(mx *Ctx) {
	pkg := rs.mgPkg(mx)
	if pkg == nil {
		return
	}

	res := rsIssues{issues: rs.slowLint(mx, pkg)}
	mx.Store.Dispatch(res)
	if len(res.issues) == 0 {
		mx.Log.Println(pkg.ImportPath, "saved with no issues, restarting")
		mx.Store.Dispatch(Restart{})
	}
}

func (rs *restartSupport) slowLint(mx *Ctx, pkg *build.Package) IssueSet {
	defer mx.Begin(Task{Title: "prepping margo restart"}).Done()

	cmds := []*exec.Cmd{
		exec.Command("margo.sh", "build", mx.AgentName()),
	}
	if pkg != nil && pkg.ImportPath != "margo" {
		cmds = append([]*exec.Cmd{
			exec.Command("go", "vet", "margo.sh/..."),
			exec.Command("go", "test", "-race", "margo.sh/..."),
		}, cmds...)
	}

	buf := &bytes.Buffer{}
	var err error
	for _, cmd := range cmds {
		cmd.Dir = mx.View.Dir()
		cmd.Env = mx.Env.Environ()
		cmd.Stdout = buf
		cmd.Stderr = buf
		err = cmd.Run()
		if err != nil {
			break
		}
	}

	output := buf.Bytes()
	isuOut := &IssueOut{
		Dir:      mx.View.Dir(),
		Patterns: mx.CommonPatterns(),
		Base:     Issue{Label: rs.ReducerLabel()},
	}
	isuOut.Write(output)
	isuOut.Close()
	issues := isuOut.Issues()

	if err != nil {
		mx.Log.Printf(rs.ReducerLabel()+": %s\n%s\n", err, output)
	}
	return issues
}
