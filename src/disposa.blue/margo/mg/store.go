package mg

import (
	"fmt"
	"sync"
)

var _ Dispatcher = (&Store{}).Dispatch

type Dispatcher func(Action)

type Listener func(*State)

type Store struct {
	mu        sync.Mutex
	state     *State
	listeners []*struct{ Listener }
	listener  Listener
	before    []Reducer
	use       []Reducer
	after     []Reducer
	cfg       func() EditorConfig
	ag        *Agent
}

func (sto *Store) Dispatch(act Action) {
	go sto.dispatch(act)
}

func (sto *Store) dispatch(act Action) {
	sto.mu.Lock()
	defer sto.mu.Unlock()

	sto.reduce(newCtx(sto.ag, sto.prepState(sto.state), act, sto), true)
}

func (sto *Store) syncRq(ag *Agent, rq *agentReq) {
	sto.mu.Lock()
	defer sto.mu.Unlock()

	name := rq.Action.Name
	mx := newCtx(sto.ag, sto.state, ag.createAction(name), sto)

	rs := agentRes{Cookie: rq.Cookie}
	rs.State = mx.State
	defer func() { ag.send(rs) }()

	if mx.Action == nil {
		rs.Error = fmt.Sprintf("unknown client action: %s", name)
		return
	}

	// TODO: add support for unpacking Action.Data

	mx = rq.Props.updateCtx(mx)
	mx.State = sto.prepState(mx.State)
	rs.State = sto.reduce(mx, false)
}

func (sto *Store) reduce(mx *Ctx, callListener bool) *State {
	apply := func(rl []Reducer) {
		for _, r := range rl {
			mx = mx.Copy(func(mx *Ctx) {
				mx.State = r.Reduce(mx)
			})
		}
	}
	apply(sto.before)
	apply(sto.use)
	apply(sto.after)

	if callListener && sto.listener != nil {
		sto.listener(mx.State)
	}

	for _, p := range sto.listeners {
		p.Listener(mx.State)
	}

	sto.state = mx.State

	return mx.State
}

func (sto *Store) State() *State {
	sto.mu.Lock()
	defer sto.mu.Unlock()

	return sto.state
}

func (sto *Store) prepState(st *State) *State {
	st = st.Copy()
	st.EphemeralState = EphemeralState{}
	if sto.cfg != nil {
		st.Config = sto.cfg()
	}
	return st
}

func newStore(ag *Agent, l Listener) *Store {
	return &Store{
		listener: l,
		state:    NewState(),
		ag:       ag,
	}
}

func (sto *Store) Subscribe(l Listener) (unsubscribe func()) {
	sto.mu.Lock()
	defer sto.mu.Unlock()

	p := &struct{ Listener }{l}
	sto.listeners = append(sto.listeners[:len(sto.listeners):len(sto.listeners)], p)

	return func() {
		sto.mu.Lock()
		defer sto.mu.Unlock()

		listeners := make([]*struct{ Listener }, 0, len(sto.listeners)-1)
		for _, q := range sto.listeners {
			if p != q {
				listeners = append(listeners, q)
			}
		}
		sto.listeners = listeners
	}
}

func (sto *Store) Before(reducers ...Reducer) *Store {
	return sto.useReducers(&sto.before, reducers)
}

func (sto *Store) Use(reducers ...Reducer) *Store {
	return sto.useReducers(&sto.use, reducers)
}

func (sto *Store) After(reducers ...Reducer) *Store {
	return sto.useReducers(&sto.after, reducers)
}

func (sto *Store) useReducers(p *[]Reducer, reducers []Reducer) *Store {
	sto.mu.Lock()
	defer sto.mu.Unlock()

	l := *p
	*p = append(l[:len(l):len(l)], reducers...)
	return sto
}

func (sto *Store) EditorConfig(f func() EditorConfig) *Store {
	sto.mu.Lock()
	defer sto.mu.Unlock()

	sto.cfg = f
	return sto
}
