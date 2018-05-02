package mg

import (
	"context"
	"fmt"
	"github.com/ugorji/go/codec"
	"margo.sh/misc/pf"
	"reflect"
	"sync"
	"time"
)

var (
	// ErrNoSettings is the error returned from EditorProps.Settings()
	// when there was no settings sent from the editor
	ErrNoSettings = fmt.Errorf("no editor settings")

	_ context.Context = (*Ctx)(nil)
)

// Ctx holds data about the current request/reduction.
//
// To create a new instance, use Store.NewCtx()
//
// NOTE: Ctx should be treated as readonly and users should not assign to any
// of its fields or the fields of any of its members.
// If a field must be updated, you should use one of the methods like Copy
//
// Unless a field is tagged with `mg.Nillable:"true"`, it will never be nil
// and if updated, no field should be set to nil
type Ctx struct {
	// State is the current state of the world
	*State

	// Action is the action that was dispatched.
	// It's a hint telling reducers about some action that happened,
	// e.g. that the view is about to be saved or that it was changed.
	Action Action `mg.Nillable:"true"`

	// Store is the global store
	Store *Store

	// Log is the global logger
	Log *Logger

	Cookie string

	Profile *pf.Profile

	doneC      chan struct{}
	cancelOnce *sync.Once
	handle     codec.Handle
}

// newCtx creates a new Ctx
// if st is nil, the state will be set to the equivalent of Store.state.new()
// if p is nil a new Profile will be created with cookie as its name
func newCtx(sto *Store, st *State, act Action, cookie string, p *pf.Profile) *Ctx {
	if st == nil {
		st = sto.state.new()
	}
	if p == nil {
		p = pf.NewProfile(cookie)
	}
	return &Ctx{
		State:      st,
		Action:     act,
		Store:      sto,
		Log:        sto.ag.Log,
		Cookie:     cookie,
		Profile:    p,
		doneC:      make(chan struct{}),
		cancelOnce: &sync.Once{},
		handle:     sto.ag.handle,
	}
}

// Deadline implements context.Context.Deadline
func (*Ctx) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

// Cancel cancels the ctx by arranging for the Ctx.Done() channel to be closed.
// Canceling this Ctx cancels all other Ctxs Copy()ed from it.
func (mx *Ctx) Cancel() {
	mx.cancelOnce.Do(func() {
		close(mx.doneC)
	})
}

// Done implements context.Context.Done()
func (mx *Ctx) Done() <-chan struct{} {
	return mx.doneC
}

// Err implements context.Context.Err()
func (mx *Ctx) Err() error {
	select {
	case <-mx.Done():
		return context.Canceled
	default:
		return nil
	}
}

// Value implements context.Context.Value() but always returns nil
func (mx *Ctx) Value(k interface{}) interface{} {
	return nil
}

// AgentName returns the name of the agent if set
// if set, it's usually the agent name as used in the command `margo.sh [run...] $agent`
func (mx *Ctx) AgentName() string {
	return mx.Store.ag.Name
}

// ActionIs returns true if the type Ctx.Action is the same type as any of those in actions
func (mx *Ctx) ActionIs(actions ...Action) bool {
	typ := reflect.TypeOf(mx.Action)
	for _, act := range actions {
		if reflect.TypeOf(act) == typ {
			return true
		}
	}
	return false
}

// LangIs is a wrapper around Ctx.View.Lang()
func (mx *Ctx) LangIs(names ...string) bool {
	return mx.View.LangIs(names...)
}

// Copy create a shallow copy of the Ctx.
//
// It applies the functions in updaters to the new object.
// Updating the new Ctx via these functions is preferred to assigning to the new object
func (mx *Ctx) Copy(updaters ...func(*Ctx)) *Ctx {
	x := *mx
	mx = &x

	for _, f := range updaters {
		f(mx)
	}
	return mx
}

func (mx *Ctx) SetState(st *State) *Ctx {
	mx = mx.Copy()
	mx.State = st
	return mx
}

func (mx *Ctx) SetView(v *View) *Ctx {
	return mx.SetState(mx.State.SetView(v))
}

// Begin stars a new task and returns its ticket
func (mx *Ctx) Begin(t Task) *TaskTicket {
	return mx.Store.Begin(t)
}

// EditorProps holds data about the text editor
type EditorProps struct {
	// Name is the name of the editor
	Name string

	// Version is the editor's version
	Version string

	handle   codec.Handle `mg.Nillable:"true"`
	settings codec.Raw
}

// Settings unmarshals the internal settings sent from the editor into v.
// If no settings were sent, it returns ErrNoSettings,
// otherwise it returns any error from unmarshalling.
func (ep *EditorProps) Settings(v interface{}) error {
	if ep.handle == nil || len(ep.settings) == 0 {
		return ErrNoSettings
	}
	return codec.NewDecoderBytes(ep.settings, ep.handle).Decode(v)
}

// EditorConfig is the common interface between internally supported editors.
//
// The main implementation is `sublime.Config`
type EditorConfig interface {
	// EditorConfig returns data to be sent to the editor.
	EditorConfig() interface{}

	// EnabledForLangs is a hint to the editor listing the languages
	// for which actions should be dispatched.
	//
	// To request actions for all languages, use `"*"` (the default)
	EnabledForLangs(langs ...string) EditorConfig
}

// StickyState is state that's persisted from one reduction to the next.
// It holds the current state of the editor.
//
// All fields are readonly and should only be assigned to during a call to State.Copy().
// Child fields esp. View should not be assigned to.
type StickyState struct {
	// View describes the current state of the view.
	// When constructed correctly (through Store.NewCtx()), View is never nil.
	View *View

	// Env holds environment variables sent from the editor.
	// For "go" views in the "margo.sh" tree and "margo" package,
	// "GOPATH" is set to the GOPATH that was used to build the agent.
	Env EnvMap

	// Editor holds data about the editor
	Editor EditorProps

	// Config holds config data for the editor to use
	Config EditorConfig `mg.Nillable:"true"`
}

// State holds data about the state of the editor, and transformations made by reducers
//
// All fields are readonly and should only be assigned to during a call to State.Copy()
// Methods on this object that return *State, return a new object.
// As an optimization/implementation details, the methods may choose to return
// the input state object if no updates are done.
//
// New instances can be obtained through Store.NewCtx()
//
// Except StickyState, all fields are cleared at the start of a new dispatch.
// Fields that to be present for some time, e.g. Status and Issues,
// Should be populated at each call to the reducer
// even if the action is not its primary action.
// e.g. for linters, they should kill off a goroutine to do a compilation
// after the file has been saved (ViewSaved) but always return its cached issues.
//
// If a reducer fails to return their state unless their primary action is dispatched
// it could result in flickering in the editor for visible elements like the status
type State struct {
	// StickyState holds the current state of the editor
	StickyState

	// Status holds the list of status messages to show in the view
	Status StrSet

	// Errors hold the list of error to display to the user
	Errors StrSet

	// Completions holds the list of completions to show to the user
	Completions []Completion

	// Issues holds the list of issues to present to the user
	Issues IssueSet

	// BuiltinCmds holds the list of builtin commands.
	// It's usually populated during the RunCmd action.
	BuiltinCmds BultinCmdList

	// UserCmds holds the list of user commands.
	// It's usually populated during the QueryUserCmds action.
	UserCmds []UserCmd

	// clientActions is a list of client actions to dispatch in the editor
	clientActions []clientActionType
}

// ActionLabel returns a label for the actions act.
// It takes into account mg.Render being an alias for nil.
func ActionLabel(act Action) string {
	t := reflect.TypeOf(act)
	if t != nil {
		if s := act.ActionLabel(); s != "" {
			return s
		}
		return t.String()
	}
	return "mg.Render"
}

// newState create a new State object ensuring View is initialized correctly.
func newState(sto *Store) *State {
	return &State{
		StickyState: StickyState{View: newView(sto)},
	}
}

// new creates a new State sharing State.StickyState
func (st *State) new() *State {
	return &State{StickyState: st.StickyState}
}

// Copy create a shallow copy of the State.
//
// It applies the functions in updaters to the new object.
// Updating the new State via these functions is preferred to assigning to the new object
func (st *State) Copy(updaters ...func(*State)) *State {
	x := *st
	st = &x

	for _, f := range updaters {
		f(st)
	}
	return st
}

// AddStatusf is equivalent to State.AddStatus(fmt.Sprintf())
func (st *State) AddStatusf(format string, a ...interface{}) *State {
	return st.AddStatus(fmt.Sprintf(format, a...))
}

// AddStatus adds the list of messages in l to State.Status.
func (st *State) AddStatus(l ...string) *State {
	if len(l) == 0 {
		return st
	}
	return st.Copy(func(st *State) {
		st.Status = st.Status.Add(l...)
	})
}

// AddErrorf is equivalent to State.AddError(fmt.Sprintf())
func (st *State) AddErrorf(format string, a ...interface{}) *State {
	return st.AddError(fmt.Errorf(format, a...))
}

// AddError adds the non-nil errors in l to State.Errors.
func (st *State) AddError(l ...error) *State {
	if len(l) == 0 {
		return st
	}
	return st.Copy(func(st *State) {
		for _, e := range l {
			if e != nil {
				st.Errors = st.Errors.Add(e.Error())
			}
		}
	})
}

// SetConfig updates the State.Config.
func (st *State) SetConfig(c EditorConfig) *State {
	return st.Copy(func(st *State) {
		st.Config = c
	})
}

// SetSrc is a wrapper around View.SetSrc().
// If `len(src) == 0` it does nothing because this is almost always a bug.
func (st *State) SetSrc(src []byte) *State {
	if len(src) == 0 {
		return st
	}
	return st.Copy(func(st *State) {
		st.View = st.View.SetSrc(src)
	})
}

func (st *State) SetView(v *View) *State {
	if st.View == v {
		return st
	}
	st = st.Copy()
	st.View = v
	return st
}

// AddCompletions adds the completions in l to State.Completions
func (st *State) AddCompletions(l ...Completion) *State {
	if len(l) == 0 {
		return st
	}
	return st.Copy(func(st *State) {
		st.Completions = append(st.Completions[:len(st.Completions):len(st.Completions)], l...)
	})
}

// AddIssues adds the list of issues in l to State.Issues
func (st *State) AddIssues(l ...Issue) *State {
	if len(l) == 0 {
		return st
	}
	return st.Copy(func(st *State) {
		st.Issues = st.Issues.Add(l...)
	})
}

// AddBuiltinCmds adds the list of builtin commands in l to State.BuiltinCmds
func (st *State) AddBuiltinCmds(l ...BultinCmd) *State {
	if len(l) == 0 {
		return st
	}
	return st.Copy(func(st *State) {
		st.BuiltinCmds = append(st.BuiltinCmds[:len(st.BuiltinCmds):len(st.BuiltinCmds)], l...)
	})
}

// AddUserCmds adds the list of user commands in l to State.userCmds
func (st *State) AddUserCmds(l ...UserCmd) *State {
	if len(l) == 0 {
		return st
	}
	return st.Copy(func(st *State) {
		st.UserCmds = append(st.UserCmds[:len(st.UserCmds):len(st.UserCmds)], l...)
	})
}

// addClientActions adds the list of client actions in l to State.clientActions
func (st *State) addClientActions(l ...clientAction) *State {
	if len(l) == 0 {
		return st
	}
	return st.Copy(func(st *State) {
		el := make([]clientActionType, 0, len(st.clientActions)+len(l))
		el = append(el, st.clientActions...)
		for _, ca := range l {
			el = append(el, ca.clientAction())
		}
		st.clientActions = el
	})
}

type clientProps struct {
	Editor struct {
		EditorProps
		Settings codec.Raw
	}
	Env  EnvMap
	View *View
}

func (cp *clientProps) finalize(ag *Agent) {
	ce := &cp.Editor
	ep := &cp.Editor.EditorProps
	ep.handle = ag.handle
	ep.settings = ce.Settings
}

func makeClientProps(kvs KVStore) clientProps {
	return clientProps{
		Env:  EnvMap{},
		View: newView(kvs),
	}
}
