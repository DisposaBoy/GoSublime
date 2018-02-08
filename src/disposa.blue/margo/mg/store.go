package mg

import (
	"fmt"
	"sync"
)

type Listener func(*State)

type Store struct {
	mu        sync.Mutex
	state     *State
	listeners []*struct{ Listener }
	listener  Listener
	reducers  []Reducer
	cfg       func() EditorConfig
}

func (sto *Store) Dispatch(act Action) {
	go sto.dispatch(act)
}

func (sto *Store) dispatch(act Action) {
	sto.mu.Lock()
	defer sto.mu.Unlock()

	sto.reduce(newCtx(sto.prepState(sto.state), act, sto), true)
}

func (sto *Store) syncRq(ag *Agent, rq *agentReq) {
	sto.mu.Lock()
	defer sto.mu.Unlock()

	name := rq.Action.Name
	mx := newCtx(sto.state, ag.createAction(name), sto)

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
	for _, r := range sto.reducers {
		mx = mx.Copy(func(mx *Ctx) {
			mx.State = r.Reduce(mx)
		})
	}

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

func newStore(l Listener) *Store {
	return &Store{listener: l, state: NewState()}
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

func (sto *Store) Use(reducers ...Reducer) *Store {
	sto.mu.Lock()
	defer sto.mu.Unlock()

	sto.reducers = append(sto.reducers[:len(sto.reducers):len(sto.reducers)], reducers...)
	return sto
}

func (sto *Store) EditorConfig(f func() EditorConfig) *Store {
	sto.mu.Lock()
	defer sto.mu.Unlock()

	sto.cfg = f
	return sto
}
