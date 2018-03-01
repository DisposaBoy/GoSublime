package golang

import (
	"bytes"
	"disposa.blue/margo/mg"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"sync"
)

type LintArgs struct {
	Writer *mg.IssueWriter
	Env    mg.EnvMap
	Dir    string
}

type LintFunc func(LintArgs) error

type LinterOpts struct {
	Log      *mg.Logger
	Actions  []mg.Action
	Patterns []*regexp.Regexp
	Lint     LintFunc
	Label    string
}

type linterSupport struct {
	nCh    chan struct{}
	mu     sync.RWMutex
	dir    string
	issues mg.IssueSet
}

func (ls *linterSupport) Reduce(lo LinterOpts, mx *mg.Ctx) *mg.State {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	if mx.ActionIs(mg.Started{}) {
		ls.start(lo, mx.Store)
	}
	dir := mx.View.Dir()
	if mx.ActionIs(lo.Actions...) && IsPkgDir(dir) {
		ls.notify()
	}

	if ls.dir == dir {
		return mx.State.AddIssues(ls.issues...)
	}
	return mx.State
}

func (ls *linterSupport) Command(la LintArgs, name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Env = la.Env.Environ()
	cmd.Stdout = la.Writer
	cmd.Stderr = la.Writer
	cmd.Dir = la.Dir
	return cmd
}

func (ls *linterSupport) notify() {
	select {
	case ls.nCh <- struct{}{}:
	default:
	}
}

func (ls *linterSupport) start(lo LinterOpts, sto *mg.Store) {
	ls.nCh = make(chan struct{}, 1)
	go ls.loop(lo, sto)
}

func (ls *linterSupport) loop(lo LinterOpts, sto *mg.Store) {
	for range ls.nCh {
		st := sto.State()
		dir := st.View.Dir()
		if IsPkgDir(dir) {
			ls.lint(lo, sto.Dispatch, st, dir)
		}
	}
}

func (ls *linterSupport) lint(lo LinterOpts, dispatch mg.Dispatcher, st *mg.State, dir string) {
	defer dispatch(mg.Render)

	buf := bytes.NewBuffer(nil)
	w := &mg.IssueWriter{
		Writer:   buf,
		Patterns: lo.Patterns,
		Base:     mg.Issue{Tag: mg.IssueError, Label: lo.Label},
		Dir:      dir,
	}
	if len(w.Patterns) == 0 {
		w.Patterns = CommonPatterns
	}
	err := lo.Lint(LintArgs{
		Writer: w,
		Env:    st.Env,
		Dir:    dir,
	})
	w.Flush()
	issues := w.Issues()
	if len(issues) == 0 && err != nil {
		out := bytes.TrimSpace(buf.Bytes())
		lo.Log.Printf("golang.linterSupport: '%s' in '%s' failed: %s\n%s\n", lo.Label, dir, err, out)
	}

	ls.mu.Lock()
	ls.dir = dir
	ls.issues = issues
	ls.mu.Unlock()
}

type Linter struct {
	linterSupport

	Name    string
	Args    []string
	Label   string
	TempDir []string
}

func (lt *Linter) Reduce(mx *mg.Ctx) *mg.State {
	return lt.linterSupport.Reduce(LinterOpts{
		Log:      mx.Log,
		Actions:  []mg.Action{mg.ViewSaved{}},
		Patterns: CommonPatterns,
		Lint:     lt.lint,
		Label:    lt.Label,
	}, mx)
}

func (lt *Linter) lint(la LintArgs) error {
	if len(lt.TempDir) != 0 {
		tmpDir, err := ioutil.TempDir("", "margo.golang.Linter."+lt.Label+",")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)
		for _, k := range lt.TempDir {
			la.Env = la.Env.Add(k, tmpDir)
		}
	}
	return lt.Command(la, lt.Name, lt.Args...).Run()
}

func GoInstall(args ...string) *Linter {
	return &Linter{
		Name:  "go",
		Args:  append([]string{"install"}, args...),
		Label: "Go/Install",
	}
}

func GoInstallDiscardBinaries(args ...string) *Linter {
	lt := GoInstall(args...)
	lt.TempDir = append(lt.TempDir, "GOBIN")
	return lt
}

func GoVet(args ...string) *Linter {
	return &Linter{
		Name:  "go",
		Args:  append([]string{"vet"}, args...),
		Label: "Go/Vet",
	}
}

func GoTest(args ...string) *Linter {
	return &Linter{
		Name:  "go",
		Args:  append([]string{"test"}, args...),
		Label: "Go/Test",
	}
}
