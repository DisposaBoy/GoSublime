package mg

import (
	"context"
	"disposa.blue/margo/misc/pprof/pprofdo"
	"fmt"
	"github.com/ugorji/go/codec"
	"go/build"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"time"
)

var (
	ErrNoSettings = fmt.Errorf("no editor settings")

	_ context.Context = (*Ctx)(nil)
)

type Ctx struct {
	*State
	Action Action

	Store *Store

	Log *Logger

	Parent *Ctx
	Values map[interface{}]interface{}
	DoneC  <-chan struct{}

	handle codec.Handle
}

func (_ *Ctx) Deadline() (time.Time, bool) {
	return time.Time{}, false
}

func (mx *Ctx) Done() <-chan struct{} {
	return mx.DoneC
}

func (_ *Ctx) Err() error {
	return nil
}

func (mx *Ctx) Value(k interface{}) interface{} {
	if v, ok := mx.Values[k]; ok {
		return v
	}
	if mx.Parent != nil {
		return mx.Parent.Value(k)
	}
	return nil
}

func newCtx(ag *Agent, st *State, act Action, sto *Store) (mx *Ctx, done chan struct{}) {
	if st == nil {
		panic("newCtx: state must not be nil")
	}
	if st == nil {
		panic("newCtx: store must not be nil")
	}
	done = make(chan struct{})
	return &Ctx{
		State:  st,
		Action: act,

		Store: sto,

		Log: ag.Log,

		DoneC: done,

		handle: ag.handle,
	}, done
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
	x.Parent = mx
	if len(mx.Values) != 0 {
		x.Values = make(map[interface{}]interface{}, len(mx.Values))
		for k, v := range mx.Values {
			x.Values[k] = v
		}
	}
	for _, f := range updaters {
		f(&x)
	}
	return &x
}

func (mx *Ctx) Begin(t Task) *TaskTicket {
	return mx.Store.Begin(t)
}

type Reducer interface {
	Reduce(*Ctx) *State
}

type ReducerList []Reducer

func (rl ReducerList) ReduceCtx(mx *Ctx) *Ctx {
	for _, r := range rl {
		var st *State
		pprofdo.Do(mx, rl.labels(r), func(_ context.Context) {
			st = r.Reduce(mx)
		})
		mx = mx.Copy(func(mx *Ctx) {
			mx.State = st
		})
	}
	return mx
}

func (rl ReducerList) labels(r Reducer) []string {
	lbl := ""
	if rf, ok := r.(ReduceFunc); ok {
		lbl = rf.Label
	} else {
		lbl = reflect.TypeOf(r).String()
	}
	return []string{"margo.reduce", lbl}
}

func (rl ReducerList) Reduce(mx *Ctx) *State {
	return rl.ReduceCtx(mx).State
}

func (rl ReducerList) Add(reducers ...Reducer) ReducerList {
	return append(rl[:len(rl):len(rl)], reducers...)
}

type ReduceFunc struct {
	Func  func(*Ctx) *State
	Label string
}

func (rf ReduceFunc) Reduce(mx *Ctx) *State {
	return rf.Func(mx)
}

func Reduce(f func(*Ctx) *State) ReduceFunc {
	_, fn, line, _ := runtime.Caller(1)
	for _, gp := range strings.Split(build.Default.GOPATH, string(filepath.ListSeparator)) {
		s := strings.TrimPrefix(fn, filepath.Clean(gp)+string(filepath.Separator))
		if s != fn {
			fn = filepath.ToSlash(s)
			break
		}
	}
	return ReduceFunc{
		Func:  f,
		Label: fmt.Sprintf("%s:%d", fn, line),
	}
}

type EditorProps struct {
	Name    string
	Version string

	handle   codec.Handle
	settings codec.Raw
}

func (ep *EditorProps) Settings(v interface{}) error {
	if ep.handle == nil || len(ep.settings) == 0 {
		return ErrNoSettings
	}
	return codec.NewDecoderBytes(ep.settings, ep.handle).Decode(v)
}

type EditorConfig interface {
	EditorConfig() interface{}
	EnabledForLangs(langs ...string) EditorConfig
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
	View   *View
	Env    EnvMap
	Editor EditorProps

	clientActions []clientAction
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

func (st *State) addClientActions(l ...clientAction) *State {
	return st.Copy(func(st *State) {
		el := st.clientActions
		st.clientActions = append(el[:len(el):len(el)], l...)
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

func makeClientProps() clientProps {
	return clientProps{
		Env:  EnvMap{},
		View: &View{},
	}
}

func (c *clientProps) updateCtx(mx *Ctx) *Ctx {
	return mx.Copy(func(mx *Ctx) {
		mx.State = mx.State.Copy(func(st *State) {
			st.Editor = c.Editor.EditorProps
			if c.Env != nil {
				st.Env = c.Env
			}
			if c.View != nil {
				st.View = c.View
				// TODO: convert View.Pos to bytes
				// at moment gocode is most affected,
				// but to fix it here means we have to read the file off-disk
				// so I'd rather not do that until we have some caching in place
			}
			if st.View != nil {
				osGopath := os.Getenv("GOPATH")
				fn := st.View.Filename()
				for _, dir := range strings.Split(osGopath, string(filepath.ListSeparator)) {
					if IsParentDir(dir, fn) {
						st.Env = st.Env.Add("GOPATH", osGopath)
						break
					}
				}
			}
		})
	})
}
