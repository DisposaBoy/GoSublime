package mg

import (
	"fmt"
	"github.com/ugorji/go/codec"
	"margo.sh/htm"
	"margo.sh/mg/actions"
	"reflect"
)

var (
	// ErrNoSettings is the error returned from EditorProps.Settings()
	// when there was no settings sent from the editor
	ErrNoSettings = fmt.Errorf("no editor settings")
)

type EditorClientProps struct {
	// Name is the name of the client
	Name string

	// Tag is the client's version
	Tag string
}

// EditorProps holds data about the text editor
type EditorProps struct {
	// Name is the name of the editor
	Name string

	// Version is the editor's version
	Version string

	// Client hold details about client (the editor plugin)
	Client EditorClientProps

	handle   codec.Handle `mg.Nillable:"true"`
	settings codec.Raw
}

// Ready returns true if the editor state has synced
//
// Reducers can call Ready in their RCond method to avoid mounting until
// the editor has communicated its state.
// Before the editor is ready, State.View, State.Editor, etc. might not contain usable data.
func (ep *EditorProps) Ready() bool {
	return ep.Name != ""
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
	EnabledForLangs(langs ...Lang) EditorConfig
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
	BuiltinCmds BuiltinCmdList

	// UserCmds holds the list of user commands.
	// It's usually populated during the QueryUserCmds and QueryTestCmds actions.
	UserCmds UserCmdList

	// Tooltips is a list of tips to show the user
	Tooltips []Tooltip

	// HUD contains information to the displayed to the user
	HUD HUDState

	// clientActions is a list of client actions to dispatch in the editor
	clientActions []actions.ClientData
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

// AddHUD adds a new article to State.HUD
func (st *State) AddHUD(heading htm.IElement, content ...htm.Element) *State {
	return st.Copy(func(st *State) {
		st.HUD = st.HUD.AddArticle(heading, content...)
	})
}

// AddTooltips add the list of tooltips l to State.Tooltips
func (st *State) AddTooltips(l ...Tooltip) *State {
	if len(l) == 0 {
		return st
	}
	return st.Copy(func(st *State) {
		st.Tooltips = append(st.Tooltips[:len(st.Tooltips):len(st.Tooltips)], l...)
	})
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

// SetEnv updates State.Env.
func (st *State) SetEnv(m EnvMap) *State {
	return st.Copy(func(st *State) {
		st.Env = m
	})
}

// SetSrc is a wrapper around View.SetSrc().
// If `len(src) == 0` it does nothing because this is almost always a bug.
func (st *State) SetViewSrc(src []byte) *State {
	if len(src) == 0 {
		return st
	}
	return st.SetView(st.View.SetSrc(src))
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
func (st *State) AddBuiltinCmds(l ...BuiltinCmd) *State {
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
func (st *State) addClientActions(l ...actions.ClientAction) *State {
	if len(l) == 0 {
		return st
	}
	return st.Copy(func(st *State) {
		el := make([]actions.ClientData, 0, len(st.clientActions)+len(l))
		el = append(el, st.clientActions...)
		for _, ca := range l {
			el = append(el, ca.ClientAction())
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
