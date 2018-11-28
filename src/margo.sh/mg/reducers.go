package mg

import (
	"reflect"
	"runtime"
	"sync"
)

var (
	// DefaultReducers enables the automatic registration of reducers to the Agent's store
	//
	// This can be used to register reducers without user-interaction
	// but where possible, it should not be used.
	//
	// its methods should only be callsed during init()
	// any calls after this may ignored
	DefaultReducers = &defaultReducers{
		before: reducerList{
			&issueKeySupport{},
			Builtins,
		},
		after: reducerList{
			&issueStatusSupport{},
			&cmdSupport{},
			&restartSupport{},
			&clientActionSupport{},
		},
	}

	nopReducer = NewReducer(func(mx *Ctx) *State { return mx.State })
)

type defaultReducers struct {
	mu                 sync.Mutex
	before, use, after reducerList
}

// Before arranges for the reducers in l to be registered when the agent starts
// it's the equivalent of the user manually calling Store.Before(l...)
func (dr *defaultReducers) Before(l ...Reducer) {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	dr.before = dr.before.Add(l...)
}

// Use arranges for the reducers in l to be registered when the agent starts
// it's the equivalent of the user manually calling Store.Use(l...)
func (dr *defaultReducers) Use(l ...Reducer) {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	dr.use = dr.use.Add(l...)
}

// After arranges for the reducers in l to be registered when the agent starts
// it's the equivalent of the user manually calling Store.After(l...)
func (dr *defaultReducers) After(l ...Reducer) {
	dr.mu.Lock()
	defer dr.mu.Unlock()

	dr.after = dr.after.Add(l...)
}

// A Reducer is the main method of state transitions in margo.
//
// The methods are called in the order listed below:
//
// * RInit
//   this is called during the first action (initAction{} FKA Started{})
//
// * RConfig
//   this is called on each reduction
//
// * RCond
//   this is called on each reduction
//   if it returns false, no other method is called
//
// * RMount
//   this is called once, after the first time RCond returns true
//
// * Reduce
//   this is called on each reduction until the agent begins shutting down
//
// * RUnmount
//   this is called once when the agent is shutting down,
//   iif RMount was called
//
// For simplicity and the ability to extend the interface in the future,
// users should embed `ReducerType` in their types to complete the interface.
//
// For convenience, it also implements all optional (non-Reduce()) methods.
//
// The method prefix `^R[A-Z]\w+` and name `Reduce` are reserved, and should not be used.
//
// For backwards compatibility the legacy methods:
// ReducerInit, ReducerConfig, ReducerCond, ReducerMount and ReducerUnmount
// will be called if the reducer does *not* defined the corresponding lifecycle method.
// i.e. if a reducer defines `ReducerInit` but not `RInit`, `ReducerInit` will be called.
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

	// RLabel returns a string that can be used to name the reducer
	// in pf.Profile and other display scenarios
	RLabel() string
	ReducerLabel() string

	// RInit is called for the first reduction
	// * it's only called once and can be used to initialise reducer state
	//   e.g. for initialising an embedded type
	// * it's called before RConfig()
	RInit(*Ctx)
	ReducerInit(*Ctx)

	// RConfig is called on each reduction, before RCond
	// if it returns a new EditorConfig, it's equivalent to State.SetConfig()
	// but is always run before RCond() so is usefull for making sure
	// configuration changes are always applied, even if Reduce() isn't called
	RConfig(*Ctx) EditorConfig
	ReducerConfig(*Ctx) EditorConfig

	// RCond is called before Reduce and RMount is called
	// if it returns false, no other methods are called
	//
	// It can be used as a pre-condition in combination with Reducer(Un)Mount
	RCond(*Ctx) bool
	ReducerCond(*Ctx) bool

	// RMount is called once, after the first time that RCond returns true
	RMount(*Ctx)
	ReducerMount(*Ctx)

	// RUnmount is called when communication with the client will stop
	// it is only called if RMount was called
	//
	// It can be used to clean up any resources created in RMount
	//
	// After this method is called, Reduce will never be called again
	RUnmount(*Ctx)
	ReducerUnmount(*Ctx)

	reducerType() *ReducerType
}

// ReducerType implements all optional methods of a reducer
type ReducerType struct {
	parent    Reducer
	mounted   bool
	unmounted bool
}

// RLabel implements Reducer.RLabel
func (rt *ReducerType) RLabel() string { return rt.r().ReducerLabel() }

// ReducerLabel implements Reducer.ReducerLabel
func (rt *ReducerType) ReducerLabel() string { return "" }

// RInit implements Reducer.RInit
func (rt *ReducerType) RInit(mx *Ctx) { rt.r().ReducerInit(mx) }

// ReducerInit implements Reducer.ReducerInit
func (rt *ReducerType) ReducerInit(*Ctx) {}

// RCond implements Reducer.RCond
func (rt *ReducerType) RCond(mx *Ctx) bool { return rt.r().ReducerCond(mx) }

// ReducerCond implements Reducer.ReducerCond
func (rt *ReducerType) ReducerCond(*Ctx) bool { return true }

// RConfig implements Reducer.RConfig
func (rt *ReducerType) RConfig(mx *Ctx) EditorConfig {
	return rt.r().ReducerConfig(mx)
}

// ReducerConfig implements Reducer.ReducerConfig
func (rt *ReducerType) ReducerConfig(*Ctx) EditorConfig { return nil }

// RMount implements Reducer.RMount
func (rt *ReducerType) RMount(mx *Ctx) { rt.r().ReducerMount(mx) }

// ReducerMount implements Reducer.ReducerMount
func (rt *ReducerType) ReducerMount(*Ctx) {}

// RUnmount implements Reducer.RUnmount
func (rt *ReducerType) RUnmount(mx *Ctx) { rt.r().ReducerUnmount(mx) }

// ReducerUnmount implements Reducer.ReducerUnmount
func (rt *ReducerType) ReducerUnmount(*Ctx) {}

func (rt *ReducerType) r() Reducer {
	if rt.parent != nil {
		return rt.parent
	}
	return nopReducer
}

func (rt *ReducerType) reducerType() *ReducerType { return rt }

func (rt *ReducerType) bootstrap(parent Reducer) {
	switch {
	case rt.parent == nil:
		rt.parent = parent
	case rt.parent != parent:
		panic("impossibru!")
	}
}

func (rt *ReducerType) reduction(mx *Ctx, r Reducer) *Ctx {
	rt.bootstrap(r)

	defer mx.Profile.Push(ReducerLabel(r)).Pop()

	rt.init(mx)

	if c := rt.config(mx); c != nil {
		mx = mx.SetState(mx.State.SetConfig(c))
	}

	if !rt.cond(mx) {
		// if mount was called, unmount must be called, even if cond returns false
		rt.unmount(mx)
		return mx
	}

	rt.mount(mx)

	if rt.unmount(mx) {
		return mx
	}

	return rt.reduce(mx)
}

func (rt *ReducerType) init(mx *Ctx) {
	if _, ok := mx.Action.(initAction); !ok {
		return
	}

	defer mx.Profile.Push("Init").Pop()
	rt.r().RInit(mx)
}

func (rt *ReducerType) config(mx *Ctx) EditorConfig {
	defer mx.Profile.Push("Config").Pop()
	return rt.r().RConfig(mx)
}

func (rt *ReducerType) cond(mx *Ctx) bool {
	defer mx.Profile.Push("Cond").Pop()
	return rt.r().RCond(mx)
}

func (rt *ReducerType) mount(mx *Ctx) {
	if rt.mounted {
		return
	}

	defer mx.Profile.Push("Mount").Pop()
	rt.mounted = true
	rt.r().RMount(mx)
}

func (rt *ReducerType) unmount(mx *Ctx) bool {
	if !rt.mounted || rt.unmounted || !mx.ActionIs(unmount{}) {
		return false
	}

	defer mx.Profile.Push("Unmount").Pop()
	rt.unmounted = true
	rt.r().RUnmount(mx)
	return true
}

func (rt *ReducerType) reduce(mx *Ctx) *Ctx {
	defer mx.Profile.Push("Reduce").Pop()
	return mx.SetState(rt.r().Reduce(mx))
}

// Add adds new reducers to the list. It returns a new list.
func (rl reducerList) Add(reducers ...Reducer) reducerList {
	return append(rl[:len(rl):len(rl)], reducers...)
}

// reducerList is a slice of reducers
type reducerList []Reducer

func (rl reducerList) reduction(mx *Ctx) *Ctx {
	for _, r := range rl {
		mx = r.reducerType().reduction(mx, r)
	}
	return mx
}

// RFunc wraps a function to be used as a reducer
// New instances should ideally be created using the global NewReducer() function
type RFunc struct {
	ReducerType

	// Label is an optional string that may be used as a name for the reducer.
	// If unset, a name based on the Func type will be used.
	Label string

	// Func is the equivalent of Reducer.Reduce
	// If Func is nil, the current state is returned as-is
	Func ReduceFn

	// The following optional fields correspond with the Reducer lifecycle methods

	// Init is the equivalent of Reducer.RInit
	Init func(mx *Ctx)

	// Cond is the equivalent of Reducer.RCond
	Cond func(mx *Ctx) bool

	// RCnfig is the equivalent of Reducer.RConfig
	Config func(mx *Ctx) EditorConfig

	// Rount is the equivalent of Reducer.RMount
	Mount func(mx *Ctx)

	// RUnount is the equivalent of Reducer.RUnmount
	Unmount func(mx *Ctx)
}

// ReduceFunc is an alias for RFunc
type ReduceFunc = RFunc

// RLabel implements Reducer.RLabel
func (rf *RFunc) RLabel() string {
	if s := rf.Label; s != "" {
		return s
	}
	nm := ""
	if rf.Func != nil {
		p := runtime.FuncForPC(reflect.ValueOf(rf.Func).Pointer())
		if p == nil {
			nm = p.Name()
		}
	}
	return "mg.Reduce(" + nm + ")"
}

// RInit delegates to RFunc.Init if it's not nil
func (rf *RFunc) RInit(mx *Ctx) {
	if rf.Init != nil {
		rf.Init(mx)
	} else {
		rf.ReducerType.RInit(mx)
	}
}

// RCond delegates to RFunc.Cond if it's not nil
func (rf *RFunc) RCond(mx *Ctx) bool {
	if rf.Cond != nil {
		return rf.Cond(mx)
	}
	return rf.ReducerType.RCond(mx)
}

// RConfig delegates to RFunc.Config if it's not nil
func (rf *RFunc) RConfig(mx *Ctx) EditorConfig {
	if rf.Config != nil {
		return rf.Config(mx)
	}
	return rf.ReducerType.RConfig(mx)
}

// RMount delegates to RFunc.Mount if it's not nil
func (rf *RFunc) RMount(mx *Ctx) {
	if rf.Mount != nil {
		rf.Mount(mx)
	} else {
		rf.ReducerType.RMount(mx)
	}
}

// RUnmount delegates to RFunc.Unmount if it's not nil
func (rf *RFunc) RUnmount(mx *Ctx) {
	if rf.Unmount != nil {
		rf.Unmount(mx)
	} else {
		rf.ReducerType.RUnmount(mx)
	}
}

// Reduce implements the Reducer interface, delegating to RFunc.Func if it's not nil
func (rf *RFunc) Reduce(mx *Ctx) *State {
	if rf.Func != nil {
		return rf.Func(mx)
	}
	return mx.State
}

// NewReducer creates a new RFunc
// reduce can be nil, in which case RFunc.Reduce method will simply return the current state
// each function in options is called on the newly created RFunc
func NewReducer(reduce ReduceFn, options ...func(*RFunc)) *RFunc {
	rf := &RFunc{Func: reduce}
	for _, o := range options {
		o(rf)
	}
	return rf
}

// ReducerLabel returns a label for the reducer r.
// It takes into account the Reducer.RLabel method.
func ReducerLabel(r Reducer) string {
	if lbl := r.RLabel(); lbl != "" {
		return lbl
	}
	if t := reflect.TypeOf(r); t != nil {
		return t.String()
	}
	return "mg.Reducer"
}

type ReduceFn func(*Ctx) *State
