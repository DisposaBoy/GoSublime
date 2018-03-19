package mg

import (
	"fmt"
	"sync"
)

var _ Dispatcher = (&Store{}).Dispatch

type Dispatcher func(Action)

type Listener func(*State)

type storeReducers struct {
	before ReducerList
	use    ReducerList
	after  ReducerList
}

func (sr storeReducers) Reduce(mx *Ctx) *State {
	mx = sr.before.ReduceCtx(mx)
	mx = sr.use.ReduceCtx(mx)
	mx = sr.after.ReduceCtx(mx)
	return mx.State
}

func (sr storeReducers) Copy(updaters ...func(*storeReducers)) storeReducers {
	for _, f := range updaters {
		f(&sr)
	}
	return sr
}

type Store struct {
	mu        sync.Mutex
	readyCh   chan struct{}
	state     *State
	listeners []*struct{ Listener }
	listener  Listener
	reducers  struct {
		sync.Mutex
		storeReducers
	}
	cfg   func() EditorConfig
	ag    *Agent
	tasks *taskTracker
	cache struct {
		sync.RWMutex
		vName string
		vHash string
		m     map[interface{}]interface{}
	}
}

func (sto *Store) ready() {
	close(sto.readyCh)
}

func (sto *Store) Dispatch(act Action) {
	go func() {
		<-sto.readyCh
		sto.dispatch(act)
	}()
}

func (sto *Store) dispatch(act Action) {
	sto.mu.Lock()
	defer sto.mu.Unlock()

	mx, done := newCtx(sto.ag, sto.prepState(sto.state), act, sto)
	defer close(done)
	st := sto.reducers.Reduce(mx)
	sto.updateState(st, true)
}

func (sto *Store) syncRq(ag *Agent, rq *agentReq) {
	sto.mu.Lock()
	defer sto.mu.Unlock()

	name := rq.Action.Name
	mx, done := newCtx(sto.ag, sto.state, ag.createAction(name), sto)
	defer close(done)

	rs := agentRes{Cookie: rq.Cookie}
	rs.State = mx.State
	defer func() { ag.send(rs) }()

	if mx.Action == nil {
		rs.Error = fmt.Sprintf("unknown client action: %s", name)
		return
	}

	// TODO: add support for unpacking Action.Data

	mx = rq.Props.updateCtx(mx)
	sto.initCache(mx.View)
	mx.State = sto.prepState(mx.State)
	mx.State = sto.reducers.Reduce(mx)
	rs.State = sto.updateState(mx.State, false)
}

func (sto *Store) updateState(st *State, callListener bool) *State {
	if callListener && sto.listener != nil {
		sto.listener(st)
	}
	for _, p := range sto.listeners {
		p.Listener(st)
	}
	sto.state = st
	return st
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
	sto := &Store{
		readyCh:  make(chan struct{}),
		listener: l,
		state:    NewState(),
		ag:       ag,
	}
	sto.cache.m = map[interface{}]interface{}{}
	sto.tasks = newTaskTracker(sto.Dispatch)
	sto.After(sto.tasks)
	return sto
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

func (sto *Store) updateReducers(updaters ...func(*storeReducers)) *Store {
	sto.reducers.Lock()
	defer sto.reducers.Unlock()

	sto.reducers.storeReducers = sto.reducers.Copy(updaters...)
	return sto
}

func (sto *Store) Before(reducers ...Reducer) *Store {
	return sto.updateReducers(func(sr *storeReducers) {
		sr.before = sr.before.Add(reducers...)
	})
}

func (sto *Store) Use(reducers ...Reducer) *Store {
	return sto.updateReducers(func(sr *storeReducers) {
		sr.use = sr.use.Add(reducers...)
	})
}

func (sto *Store) After(reducers ...Reducer) *Store {
	return sto.updateReducers(func(sr *storeReducers) {
		sr.after = sr.after.Add(reducers...)
	})
}

func (sto *Store) EditorConfig(f func() EditorConfig) *Store {
	sto.mu.Lock()
	defer sto.mu.Unlock()

	sto.cfg = f
	return sto
}

func (sto *Store) Begin(t Task) *TaskTicket {
	return sto.tasks.Begin(t)
}

func (sto *Store) initCache(v *View) {
	cc := &sto.cache
	cc.Lock()
	defer cc.Unlock()

	if cc.vHash != v.Hash || cc.vName != v.Name {
		cc.m = map[interface{}]interface{}{}
	}
}

func (sto *Store) Put(k interface{}, v interface{}) {
	sto.cache.Lock()
	defer sto.cache.Unlock()

	sto.cache.m[k] = v
}

func (sto *Store) Get(k interface{}) interface{} {
	sto.cache.RLock()
	defer sto.cache.RUnlock()

	return sto.cache.m[k]
}

func (sto *Store) Del(k interface{}) {
	sto.cache.Lock()
	defer sto.cache.Unlock()

	delete(sto.cache.m, k)
}
