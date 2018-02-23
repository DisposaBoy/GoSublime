package mg

import (
	"fmt"
	"log"
	"reflect"
)

type Ctx struct {
	*State
	Action Action

	Editor EditorProps
	Env    EnvMap
	Store  *Store

	Log *log.Logger
	Dbg *log.Logger
}

func newCtx(st *State, act Action, sto *Store) *Ctx {
	if st == nil {
		panic("newCtx: state must not be nil")
	}
	if st == nil {
		panic("newCtx: store must not be nil")
	}
	return &Ctx{
		State:  st,
		Action: act,

		Store: sto,

		Log: Log,
		Dbg: Dbg,
	}
}

func (mx *Ctx) ActionIs(actions ...Action) bool {
	typ := reflect.TypeOf(mx.Action)
	for _, act := range actions {
		if reflect.TypeOf(act) == typ {
			return true
		}
	}
	return false
}

func (mx *Ctx) LangIs(names ...string) bool {
	return mx.View.LangIs(names...)
}

func (mx *Ctx) Copy(updaters ...func(*Ctx)) *Ctx {
	x := *mx
	for _, f := range updaters {
		f(&x)
	}
	return &x
}

type Reducer interface {
	Reduce(*Ctx) *State
}

type Reduce func(*Ctx) *State

func (r Reduce) Reduce(mx *Ctx) *State {
	return r(mx)
}

type EditorProps struct {
	Name    string
	Version string
}

type EditorConfig interface {
	EditorConfig() interface{}
}

type EphemeralState struct {
	Config      EditorConfig
	Status      StrSet
	Errors      StrSet
	Completions []Completion
	Tooltips    []Tooltip
	Issues      IssueSet
}

type State struct {
	EphemeralState
	View     *View
	Obsolete bool
}

func NewState() *State {
	return &State{
		View: newView(),
	}
}

func (st *State) Copy(updaters ...func(*State)) *State {
	x := *st
	for _, f := range updaters {
		f(&x)
	}
	return &x
}

func (st *State) AddStatusf(format string, a ...interface{}) *State {
	return st.AddStatus(fmt.Sprintf(format, a...))
}

func (st *State) AddStatus(l ...string) *State {
	if len(l) == 0 {
		return st
	}
	return st.Copy(func(st *State) {
		st.Status = st.Status.Add(l...)
	})
}

func (st *State) Errorf(format string, a ...interface{}) *State {
	return st.AddError(fmt.Errorf(format, a...))
}

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

func (st *State) SetConfig(c EditorConfig) *State {
	return st.Copy(func(st *State) {
		st.Config = c
	})
}

func (st *State) SetSrc(src []byte) *State {
	return st.Copy(func(st *State) {
		st.View = st.View.SetSrc(src)
	})
}

func (st *State) AddCompletions(l ...Completion) *State {
	return st.Copy(func(st *State) {
		st.Completions = append(st.Completions[:len(st.Completions):len(st.Completions)], l...)
	})
}

func (st *State) AddTooltips(l ...Tooltip) *State {
	return st.Copy(func(st *State) {
		st.Tooltips = append(st.Tooltips[:len(st.Tooltips):len(st.Tooltips)], l...)
	})
}

func (st *State) AddIssues(l ...Issue) *State {
	if len(l) == 0 {
		return st
	}
	return st.Copy(func(st *State) {
		st.Issues = st.Issues.Add(l...)
	})
}

func (st *State) MarkObsolete() *State {
	return st.Copy(func(st *State) {
		st.Obsolete = true
	})
}

type clientProps struct {
	Editor EditorProps
	Env    EnvMap
	View   *View
}

func makeClientProps() clientProps {
	return clientProps{
		Env:  EnvMap{},
		View: &View{},
	}
}

func (c *clientProps) updateCtx(mx *Ctx) *Ctx {
	return mx.Copy(func(mx *Ctx) {
		mx.Editor = c.Editor
		if c.Env != nil {
			mx.Env = c.Env
		}
		if c.View != nil {
			mx.State = mx.State.Copy(func(st *State) {
				st.View = c.View
			})
			// TODO: convert View.Pos to bytes
			// at moment gocode is most affected,
			// but to fix it here means we have to read the file off-disk
			// so I'd rather not do that until we have some caching in place
		}
	})
}
