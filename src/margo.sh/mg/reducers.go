package mg

import (
	"reflect"
	"runtime"
)

var (
	_ Reducer = &reducerType{}

	defaultReducers = struct {
		before, use, after []Reducer
	}{
		before: []Reducer{
			&issueKeySupport{},
			Builtins,
		},
		after: []Reducer{
			&issueStatusSupport{},
			&cmdSupport{},
			&restartSupport{},
			&clientActionSupport{},
		},
	}
)

// A Reducer is the main method of state transitions in margo.
//
// The methods are called in the order listed below:
//
// * ReducerInit
//   this is called during the first action (initAction{} FKA Started{})
//
// * ReducerConfig
//   this is called on each reduction
//
// * ReducerCond
//   this is called on each reduction
//   if it returns false, no other method is called
//
// * ReducerMount
//   this is called once, after the first time ReducerCond returns true
//
// * Reduce
//   this is called on each reduction until the agent begins shutting down
//
// * ReducerUnmount
//   this is called once when the agent is shutting down,
//   iif ReducerMount was called
//
// For simplicity and the ability to extend the interface in the future,
// users should embed `ReducerType` in their types to complete the interface.
//
// For convenience, it also implements all optional (non-Reduce()) methods.
//
// The prefixes `Reduce` and `Reducer` are reserved, and should not be used.
// To work-around a frequent typo, the `ReducerXXX` methods will have an alias
// `ReduceXXX` designed to fail the build if the name is used.
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

	// ReducerInit is called for the first reduction
	// * it's only called once and can be used to initialise reducer state
	//   e.g. for initialising an embedded type
	// * it's called before ReducerConfig()
	ReducerInit(*Ctx)

	// ReducerConfig is called on each reduction, before ReducerCond
	// if it returns a new EditorConfig, it's equivalent to State.SetConfig()
	// but is always run before ReducerCond() so is usefull for making sure
	// configuration changes are always applied, even if Reduce() isn't called
	ReducerConfig(*Ctx) EditorConfig

	// ReducerCond is called before Reduce and ReducerMount is called
	// if it returns false, no other methods are called
	//
	// It can be used as a pre-condition in combination with Reducer(Un)Mount
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

	reducerType() *ReducerType

	reducerPrefixTypo
}

// reducerPrefixTypo aims to fail the build when you define a method like
// ReduceCond()... which is a typo about 100% of the time.
// it should be kept in sync with Reducer
type reducerPrefixTypo interface {
	ReduceLabel(useReducerForThePrefixNotReduce)
	ReduceInit(useReducerForThePrefixNotReduce)
	ReduceConfig(useReducerForThePrefixNotReduce)
	ReduceCond(useReducerForThePrefixNotReduce)
	ReduceMount(useReducerForThePrefixNotReduce)
	ReduceUnmount(useReducerForThePrefixNotReduce)
}

type useReducerForThePrefixNotReduce struct{}

type reducerType struct{ ReducerType }

func (rt *reducerType) Reduce(mx *Ctx) *State { return mx.State }

// ReducerType implements all optional methods of a reducer
type ReducerType struct {
	reducerPrefixTypo
}

func (rt *ReducerType) reducerType() *ReducerType { return rt }

// ReducerLabel implements Reducer.ReducerLabel
func (rt *ReducerType) ReducerLabel() string { return "" }

// ReducerInit implements Reducer.ReducerInit
func (rt *ReducerType) ReducerInit(*Ctx) {}

// ReducerCond implements Reducer.ReducerCond
func (rt *ReducerType) ReducerCond(*Ctx) bool { return true }

// ReducerConfig implements Reducer.ReducerConfig
func (rt *ReducerType) ReducerConfig(*Ctx) EditorConfig { return nil }

// ReducerMount implements Reducer.ReducerMount
func (rt *ReducerType) ReducerMount(*Ctx) {}

// ReducerUnmount implements Reducer.ReducerUnmount
func (rt *ReducerType) ReducerUnmount(*Ctx) {}

// reducerList is a slice of reducers
type reducerList []Reducer

func (rl reducerList) callReducers(mx *Ctx) *Ctx {
	for _, r := range rl {
		mx = rl.callReducer(mx, r)
	}
	return mx
}

func (rl reducerList) callReducer(mx *Ctx, r Reducer) *Ctx {
	defer mx.Profile.Push(ReducerLabel(r)).Pop()

	rl.crInit(mx, r)

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

func (rl reducerList) crInit(mx *Ctx, r Reducer) {
	if _, ok := mx.Action.(initAction); !ok {
		return
	}

	defer mx.Profile.Push("ReducerInit").Pop()
	r.ReducerInit(mx)
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
	k := r.reducerType()
	if mx.Store.mounted[k] {
		return
	}

	defer mx.Profile.Push("Mount").Pop()
	mx.Store.mounted[k] = true
	r.ReducerMount(mx)
}

func (rl reducerList) crUnmount(mx *Ctx, r Reducer) bool {
	k := r.reducerType()
	if !mx.ActionIs(unmount{}) || !mx.Store.mounted[k] {
		return false
	}
	defer mx.Profile.Push("Unmount").Pop()
	delete(mx.Store.mounted, k)
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
