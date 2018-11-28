package mg

import (
	"context"
	"github.com/ugorji/go/codec"
	"margo.sh/mgpf"
	"reflect"
	"regexp"
	"sync"
	"time"
)

var (
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

	// KVMap is an in-memory cache of data with for the lifetime of the Ctx.
	*KVMap

	// Store is the global store
	Store *Store

	// Log is the global logger
	Log *Logger

	Cookie string

	Profile *mgpf.Profile

	doneC      chan struct{}
	cancelOnce *sync.Once
	handle     codec.Handle
	defr       *redFns
}

// newCtx creates a new Ctx
// if st is nil, the state will be set to the equivalent of Store.state.new()
// if p is nil a new Profile will be created with cookie as its name
func newCtx(sto *Store, st *State, act Action, cookie string, p *mgpf.Profile) *Ctx {
	if st == nil {
		st = sto.state.new()
	}
	if st.Config == nil {
		st = st.SetConfig(sto.cfg)
	}
	if p == nil {
		p = mgpf.NewProfile(cookie)
	}
	return &Ctx{
		State:      st,
		Action:     act,
		KVMap:      &KVMap{},
		Store:      sto,
		Log:        sto.ag.Log,
		Cookie:     cookie,
		Profile:    p,
		doneC:      make(chan struct{}),
		cancelOnce: &sync.Once{},
		handle:     sto.ag.handle,
		defr:       &redFns{},
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

// ActionIs returns true if Ctx.Action is the same type as any of those in actions
// for convenience, it returns true if actions is nil
func (mx *Ctx) ActionIs(actions ...Action) bool {
	if actions == nil {
		return true
	}
	typ := reflect.TypeOf(mx.Action)
	for _, act := range actions {
		if reflect.TypeOf(act) == typ {
			return true
		}
	}
	return false
}

// LangIs is equivalent to View.LangIs(langs...)
func (mx *Ctx) LangIs(langs ...Lang) bool {
	return mx.View.LangIs(langs...)
}

// CommonPatterns is equivalent to View.CommonPatterns()
func (mx *Ctx) CommonPatterns() []*regexp.Regexp {
	return mx.View.CommonPatterns()
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
	if mx.State == st {
		return mx
	}
	mx = mx.Copy()
	mx.State = st
	return mx
}

func (mx *Ctx) SetView(v *View) *Ctx {
	return mx.SetState(mx.State.SetView(v))
}

// Begin is a short-hand for Ctx.Store.Begin
func (mx *Ctx) Begin(t Task) *TaskTicket {
	return mx.Store.Begin(t)
}

func (mx *Ctx) Defer(f ReduceFn) {
	mx.defr.prepend(f)
}

type redFns struct {
	sync.RWMutex
	l []ReduceFn
}

func (r *redFns) prepend(f ReduceFn) {
	r.Lock()
	defer r.Unlock()

	r.l = append([]ReduceFn{f}, r.l...)
}

func (r *redFns) append(f ReduceFn) {
	r.Lock()
	defer r.Unlock()

	r.l = append(r.l[:len(r.l):len(r.l)], f)
}

func (r *redFns) reduction(mx *Ctx) *Ctx {
	r.RLock()
	l := r.l
	r.l = nil
	r.RUnlock()

	for _, reduce := range l {
		mx = mx.SetState(reduce(mx))
	}
	return mx
}
