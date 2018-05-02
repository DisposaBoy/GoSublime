package mg

import (
	"go/build"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
)

var (
	_ Reducer = reducerType{}

	defaultReducers = struct {
		before, use, after []Reducer
	}{
		before: []Reducer{
			&issueKeySupport{},
			Builtins,
		},
		after: []Reducer{
			issueStatusSupport{},
			&cmdSupport{},
			&restartSupport{},
		},
	}
)

// A Reducer is the main method of state transitions in margo.
//
// ReducerType can be embedded into struct types to implement all optional methods,
// i.e. the only required method is Reduce()
//
// NewReducer() can be used to convert a function to a reducer.
//
// For reducers that are backed by goroutines that are only interested
// in the *last* of some value e.g. *Ctx, mgutil.ChanQ might be of use.
type Reducer interface {
	// Reduce takes as input a Ctx describing the current state of the world
	// and an Action describing some action that happened.
	// Based on this action, the reducer returns a new state of the world.
	//
	// Reducers are called sequentially in the order they were registered
	// with Store.Before(), Store.Use() or Store.After().
	//
	// A reducer should not call Store.State().
	//
	// Reducers should complete their work as quickly as possible,
	// ideally only updating the state and not doing any work in the reducer itself.
	//
	// If a reducer is slow it might block the editor UI because some actions like
	// fmt'ing the view must wait for the new src before the user
	// can continue editing or saving the file.
	//
	// e.g. during the ViewFmt or ViewPreSave action, a reducer that knows how to
	// fmt the file might update the state to hold a fmt'd copy of the view's src.
	//
	// or it can implement a linter that kicks off a goroutine to try to compile
	// a package when one of its files when the ViewSaved action is dispatched.
	Reduce(*Ctx) *State

	// ReducerLabel returns a string that can be used to name the reducer
	// in pf.Profile and other display scenarios
	ReducerLabel() string

	// ReducerConfig is called on each reduction, before ReducerCond
	// if it returns a new EditorConfig, it's equivalent to State.SetConfig()
	// but is always run before ReducerCond() so is usefull for making sure
	// configuration changes are always applied, even if Reduce() isn't called
	ReducerConfig(*Ctx) EditorConfig

	// ReducerCond is called before Reduce is called
	// if it returns false, Reduce is not called
	//
	// It can be used a pre-condition in combination with Reducer(Un)Mount
	ReducerCond(*Ctx) bool

	// ReducerMount is called once, after the first time that ReducerCond returns true
	ReducerMount(*Ctx)

	// ReducerUnmount is called when communication with the client will stop
	// it is only called if ReducerMount was called
	//
	// It can be used to clean up any resources created in ReducerMount
	//
	// After this method is called, Reduce will never be called again
	ReducerUnmount(*Ctx)

	reducerType() reducerType
}

type reducerType struct{ ReducerType }

func (rt reducerType) Reduce(mx *Ctx) *State { return mx.State }

// ReducerType implements all optional methods of a reducer
type ReducerType struct{}

func (rt ReducerType) reducerType() reducerType { return reducerType{} }

// ReducerLabel implements Reducer.ReducerLabel
func (rt ReducerType) ReducerLabel() string { return "" }

// ReducerCond implements Reducer.ReducerCond
func (rt ReducerType) ReducerCond(*Ctx) bool { return true }

// ReducerConfig implements Reducer.ReducerConfig
func (rt ReducerType) ReducerConfig(*Ctx) EditorConfig { return nil }

// ReducerMount implements Reducer.ReducerMount
func (rt ReducerType) ReducerMount(*Ctx) {}

// ReducerUnmount implements Reducer.ReducerUnmount
func (rt ReducerType) ReducerUnmount(*Ctx) {}

// reducerList is a slice of reducers
type reducerList []Reducer

// callReducers calls the reducers in the slice in order.
func (rl reducerList) callReducers(mx *Ctx) *Ctx {
	for _, r := range rl {
		mx = rl.callReducer(mx, r)
	}
	return mx
}

func (rl reducerList) callReducer(mx *Ctx, r Reducer) *Ctx {
	defer mx.Profile.Push(ReducerLabel(r)).Pop()
	if c := rl.crConfig(mx, r); c != nil {
		mx = mx.SetState(mx.State.SetConfig(c))
	}
	if !rl.crCond(mx, r) {
		return mx
	}
	rl.crMount(mx, r)
	if rl.crUnmount(mx, r) {
		return mx
	}
	return rl.crReduce(mx, r)
}

func (rl reducerList) crConfig(mx *Ctx, r Reducer) EditorConfig {
	defer mx.Profile.Push("ReducerConfig").Pop()
	return r.ReducerConfig(mx)
}

func (rl reducerList) crCond(mx *Ctx, r Reducer) bool {
	defer mx.Profile.Push("ReducerCond").Pop()
	return r.ReducerCond(mx)
}

func (rl reducerList) crMount(mx *Ctx, r Reducer) {
	if mx.Store.mounted[r] {
		return
	}

	defer mx.Profile.Push("Mount").Pop()
	mx.Store.mounted[r] = true
	r.ReducerMount(mx)
}

func (rl reducerList) crUnmount(mx *Ctx, r Reducer) bool {
	if !mx.ActionIs(unmount{}) || !mx.Store.mounted[r] {
		return false
	}
	defer mx.Profile.Push("Unmount").Pop()
	delete(mx.Store.mounted, r)
	r.ReducerUnmount(mx)
	return true
}

func (rl reducerList) crReduce(mx *Ctx, r Reducer) *Ctx {
	defer mx.Profile.Push("Reduce").Pop()
	return mx.SetState(r.Reduce(mx))
}

// Add adds new reducers to the list. It returns a new list.
func (rl reducerList) Add(reducers ...Reducer) reducerList {
	return append(rl[:len(rl):len(rl)], reducers...)
}

// ReduceFunc wraps a function to be used as a reducer
// New instances should ideally be created using the global NewReducer() function
type ReduceFunc struct {
	ReducerType

	// Func is the function to be used for the reducer
	Func func(*Ctx) *State

	// Label is an optional string that may be used as a name for the reducer.
	// If unset, a name based on the Func type will be used.
	Label string
}

// ReducerLabel implements ReducerLabeler
func (rf *ReduceFunc) ReducerLabel() string {
	if s := rf.Label; s != "" {
		return s
	}
	nm := ""
	if p := runtime.FuncForPC(reflect.ValueOf(rf.Func).Pointer()); p != nil {
		nm = p.Name()
	}
	return "mg.Reduce(" + nm + ")"
}

// Reduce implements the Reducer interface, delegating to ReduceFunc.Func
func (rf *ReduceFunc) Reduce(mx *Ctx) *State {
	return rf.Func(mx)
}

// NewReducer creates a new ReduceFunc
func NewReducer(f func(*Ctx) *State) *ReduceFunc {
	return &ReduceFunc{Func: f}
}

// ReducerLabel returns a label for the reducer r.
// It takes into account the ReducerLabeler interface.
func ReducerLabel(r Reducer) string {
	if lbl := r.ReducerLabel(); lbl != "" {
		return lbl
	}
	if t := reflect.TypeOf(r); t != nil {
		return t.String()
	}
	return "mg.Reducer"
}

type rsBuildRes struct {
	ActionType
	issues IssueSet
}

type restartSupport struct {
	ReducerType
	issues IssueSet
}

func (r *restartSupport) Reduce(mx *Ctx) *State {
	st := mx.State
	switch act := mx.Action.(type) {
	case ViewSaved:
		r.tryPrepRestart(mx)
	case Restart:
		mx.Log.Printf("%T action dispatched\n", mx.Action)
		st = mx.addClientActions(clientRestart)
	case Shutdown:
		mx.Log.Printf("%T action dispatched\n", mx.Action)
		st = mx.addClientActions(clientShutdown)
	case rsBuildRes:
		r.issues = act.issues
	}
	return st.AddIssues(r.issues...)
}

func (r *restartSupport) tryPrepRestart(mx *Ctx) {
	v := mx.View
	hasSfx := strings.HasSuffix
	if !hasSfx(v.Path, ".go") || hasSfx(v.Path, "_test.go") {
		return
	}

	dir := filepath.ToSlash(mx.View.Dir())
	if !filepath.IsAbs(dir) {
		return
	}

	// if we use build..ImportPath, it will be wrong if we work on the code outside the GS GOPATH
	imp := ""
	if i := strings.LastIndex(dir, "/src/"); i >= 0 {
		imp = dir[i+5:]
	}
	if imp != "margo" && !strings.HasPrefix(imp+"/", "margo.sh/") {
		return
	}

	go r.prepRestart(mx, dir)
}

func (r *restartSupport) prepRestart(mx *Ctx, dir string) {

	pkg, _ := build.Default.ImportDir(dir, 0)
	if pkg == nil || pkg.Name == "" {
		return
	}

	defer mx.Begin(Task{Title: "prepping margo restart"}).Done()

	cmd := exec.Command("margo.sh", "build", mx.AgentName())
	cmd.Dir = mx.View.Dir()
	cmd.Env = mx.Env.Environ()
	out, err := cmd.CombinedOutput()

	iw := &IssueWriter{
		Dir:      mx.View.Dir(),
		Patterns: CommonPatterns,
		Base:     Issue{Label: "Mg/RestartSupport"},
	}
	iw.Write(out)
	iw.Flush()
	res := rsBuildRes{issues: iw.Issues()}

	msg := "telling margo to restart after " + mx.View.Filename() + " was saved"
	if err == nil && len(res.issues) == 0 {
		mx.Log.Println(msg)
		mx.Store.Dispatch(Restart{})
	} else {
		mx.Log.Printf("not %s: `margo.sh build %s` failed: error: %v\n%s\n", msg, mx.AgentName(), err, out)
		mx.Store.Dispatch(res)
	}
}
