package mg

import (
	"margo.sh/mgutil"
	"os"
	"os/exec"
)

type Linter struct {
	ReducerType

	Langs   []Lang
	Actions []Action

	Name string
	Args []string

	IssueKey func(*Ctx) IssueKey
	Tag      IssueTag
	Label    string
	TempDir  []string

	q *mgutil.ChanQ
}

func (lt *Linter) RCond(mx *Ctx) bool {
	return mx.LangIs(lt.Langs...) &&
		(mx.ActionIs(lt.userActs()...) || mx.ActionIs(lt.auxActs()...))
}

func (lt *Linter) userActs() []Action {
	if acts := lt.Actions; len(acts) != 0 {
		return acts
	}
	return []Action{ViewSaved{}}
}

func (lt *Linter) auxActs() []Action {
	return []Action{QueryUserCmds{}}
}

func (lt *Linter) Reduce(mx *Ctx) *State {
	// keep non-default actions in sync with auxActs()
	switch mx.Action.(type) {
	case QueryUserCmds:
		return lt.userCmds(mx)
	default:
		lt.q.Put(mx)
		return mx.State
	}
}

func (lt *Linter) RMount(mx *Ctx) {
	lt.q = mgutil.NewChanQ(1)
	go lt.loop()
}

func (lt *Linter) RUnmount(mx *Ctx) {
	lt.q.Close()
}

func (lt *Linter) userCmds(mx *Ctx) *State {
	lbl := lt.Label
	if lbl == "" {
		lbl = lt.Name
	}
	return mx.AddUserCmds(UserCmd{
		Name:  lt.Name,
		Args:  lt.Args,
		Title: "Linter: " + lbl,
	})
}

func (lt *Linter) loop() {
	for v := range lt.q.C() {
		lt.lint(v.(*Ctx))
	}
}

func (lt *Linter) key(mx *Ctx) IssueKey {
	if ik := lt.IssueKey; ik != nil {
		return ik(mx)
	}
	return IssueKey{Key: lt}
}

func (lt *Linter) lint(mx *Ctx) {
	res := StoreIssues{}
	res.Key = lt.key(mx)
	// make sure to clear any old issues, even if we return early
	defer func() { mx.Store.Dispatch(res) }()

	cmdStr := mgutil.QuoteCmd(lt.Name, lt.Args...)
	if len(lt.TempDir) != 0 {
		tmpDir, err := MkTempDir(lt.Label)
		if err != nil {
			mx.Log.Printf("cannot create tempDir for linter `%s`: %s\n", cmdStr, err)
			return
		}
		defer os.RemoveAll(tmpDir)

		m := mx.Env
		for _, k := range lt.TempDir {
			m = m.Add(k, tmpDir)
		}
		mx = mx.SetState(mx.State.SetEnv(m))
	}

	dir := mx.View.Dir()
	iw := &IssueOut{
		Dir:      dir,
		Patterns: mx.CommonPatterns(),
		Base:     Issue{Label: lt.Label, Tag: lt.Tag},
	}

	cmd := exec.Command(lt.Name, lt.Args...)
	cmd.Stdout = iw
	cmd.Stderr = iw
	cmd.Env = mx.Env.Environ()
	cmd.Dir = dir

	if err := cmd.Start(); err != nil {
		mx.Log.Printf("cannot start linter `%s`: %s", cmdStr, err)
		return
	}
	cmd.Wait()
	iw.Close()
	res.Issues = iw.Issues()
}
