package mg

import (
	"margo.sh/mgpf"
	yotsuba "margo.sh/why_would_you_make_yotsuba_cry"
	"path/filepath"
	"strings"
	"sync"
)

var _ Dispatcher = (&Store{}).Dispatch

// Dispatcher is the signature of the Store.Dispatch method
type Dispatcher func(Action)

// Subscriber is the signature of the function accepted by Store.Subscribe
type Subscriber func(*Ctx)

type dispatchHandler func()

type storeReducers struct {
	before reducerList
	use    reducerList
	after  reducerList
}

func (sr storeReducers) Reduce(mx *Ctx) *Ctx {
	mx.Profile.Do("Before", func() {
		mx = sr.before.reduction(mx)
	})
	mx.Profile.Do("Use", func() {
		mx = sr.use.reduction(mx)
	})
	mx.Profile.Do("After", func() {
		mx = sr.after.reduction(mx)
	})
	return mx.defr.reduction(mx)
}

func (sr storeReducers) Copy(updaters ...func(*storeReducers)) storeReducers {
	for _, f := range updaters {
		f(&sr)
	}
	return sr
}

// Store holds global, shared state
type Store struct {
	// KVMap is an in-memory cache of data with automatic eviction.
	// Eviction might happen if the active view changes.
	//
	// NOTE: it's not safe to store values with *Ctx objects here; use *Ctx.KVMap instead
	KVMap

	mu       sync.Mutex
	state    *State
	subs     []*struct{ Subscriber }
	sub      Subscriber
	reducers struct {
		sync.Mutex
		storeReducers
	}
	cfg   EditorConfig `mg.Nillable:"true"`
	ag    *Agent
	tasks *taskTracker
	cache struct {
		sync.RWMutex
		vName string
		vHash string
	}

	dsp struct {
		sync.RWMutex
		lo        chan dispatchHandler
		hi        chan dispatchHandler
		unmounted bool
	}
}

func (sto *Store) mount() {
	go sto.dispatcher()
}

func (sto *Store) unmount() {
	done := make(chan struct{})
	sto.dsp.hi <- func() {
		defer close(done)

		sto.dsp.Lock()
		defer sto.dsp.Unlock()

		if sto.dsp.unmounted {
			return
		}
		sto.dsp.unmounted = true

		sto.handleAct(unmount{}, nil)
	}
	<-done
}

// Dispatch schedules a new reduction with Action act
//
// * actions coming from the editor has a higher priority
// * as a result, if Shutdown is dispatched, the action might be dropped
func (sto *Store) Dispatch(act Action) {
	c := sto.dsp.lo
	f := func() { sto.handleAct(act, nil) }
	select {
	case c <- f:
	default:
		go func() { c <- f }()
	}
}

func (sto *Store) nextDispatcher() dispatchHandler {
	var h dispatchHandler
	select {
	case h = <-sto.dsp.hi:
	default:
		select {
		case h = <-sto.dsp.hi:
		case h = <-sto.dsp.lo:
		}
	}

	sto.dsp.RLock()
	defer sto.dsp.RUnlock()

	if sto.dsp.unmounted {
		return nil
	}
	return h
}

func (sto *Store) dispatcher() {
	sto.ag.Log.Println("started")
	sto.handleAct(initAction{}, nil)

	for {
		if f := sto.nextDispatcher(); f != nil {
			f()
		} else {
			return
		}
	}
}

func (sto *Store) handleReduce(mx *Ctx) *Ctx {
	defer mx.Profile.Push("action|" + ActionLabel(mx.Action)).Pop()

	return sto.reducers.Reduce(mx)
}

func (sto *Store) handle(h func() *Ctx, p *mgpf.Profile) {
	p.Push("handleRequest")
	sto.mu.Lock()

	mx := h()
	sto.state = mx.State
	subs := sto.subs

	sto.mu.Unlock()
	p.Pop()

	for _, p := range subs {
		p.Subscriber(mx)
	}
}

func (sto *Store) handleAct(act Action, p *mgpf.Profile) {
	if p == nil {
		p = mgpf.NewProfile("")
	}
	sto.handle(func() *Ctx {
		mx := newCtx(sto, nil, act, "", p)
		return sto.handleReduce(mx)
	}, p)
}

func (sto *Store) handleReq(rq *agentReq) {
	sto.handle(func() *Ctx {
		newMx := func(st *State, act Action) *Ctx {
			return newCtx(sto, st, act, rq.Cookie, rq.Profile)
		}
		mx, acts := sto.handleReqInit(rq, newMx(nil, nil))
		for _, act := range acts {
			st := mx.State.new()
			st.Errors = mx.State.Errors
			mx = newMx(st, act)
			mx = sto.handleReduce(mx)
		}
		return mx
	}, rq.Profile)
}

func (sto *Store) handleReqInit(rq *agentReq, mx *Ctx) (*Ctx, []Action) {
	defer mx.Profile.Push("init").Pop()

	acts := make([]Action, 0, len(rq.Actions))
	for _, ra := range rq.Actions {
		act, err := sto.ag.createAction(ra)
		if err != nil {
			mx.State = mx.AddErrorf("createAction(%s): %s", ra.Name, err)
		} else {
			acts = append(acts, act)
		}
	}

	if cfg := sto.cfg; cfg != nil {
		mx.Config = cfg
	}
	props := rq.Props
	if ep := props.Editor.EditorProps; ep.Name != "" {
		mx.Editor = ep
	}
	if v := props.View; v != nil && v.Name != "" {
		mx.View = v
		sto.initCache(v)
		v.finalize()
	}
	if len(props.Env) != 0 {
		mx.Env = props.Env
	}
	mx.Env = sto.autoSwitchInternalGOPATH(mx)
	return mx, acts
}

// autoSwitchInternalGOPATH returns mx.Env with GOPATH set to the agent's GOPATH
// if mx.View.Filename is a child of said GOPATH
func (sto *Store) autoSwitchInternalGOPATH(mx *Ctx) EnvMap {
	fn := mx.View.Path
	if fn == "" {
		return mx.Env
	}
	gp := yotsuba.AgentBuildContext.GOPATH
	for _, dir := range strings.Split(gp, string(filepath.ListSeparator)) {
		if IsParentDir(dir, fn) {
			return mx.Env.Add("GOPATH", gp)
		}
	}
	return mx.Env
}

// NewCtx returns a new Ctx initialized using the internal StickyState.
// The caller is responsible for calling Ctx.Cancel() when done with the Ctx
func (sto *Store) NewCtx(act Action) *Ctx {
	sto.mu.Lock()
	defer sto.mu.Unlock()

	return newCtx(sto, nil, act, "", nil)
}

func newStore(ag *Agent, sub Subscriber) *Store {
	sto := &Store{
		sub: sub,
		ag:  ag,
	}
	sto.state = &State{
		StickyState: StickyState{View: newView(sto)},
	}
	sto.tasks = &taskTracker{}
	sto.After(sto.tasks)

	// 640 slots ought to be enough for anybody
	sto.dsp.lo = make(chan dispatchHandler, 640)
	sto.dsp.hi = make(chan dispatchHandler, 640)

	return sto
}

// Subscribe arranges for sub to be called after each reduction takes place
// the function returned can be used to unsubscribe from further notifications
func (sto *Store) Subscribe(sub Subscriber) (unsubscribe func()) {
	sto.mu.Lock()
	defer sto.mu.Unlock()

	p := &struct{ Subscriber }{sub}
	sto.subs = append(sto.subs[:len(sto.subs):len(sto.subs)], p)

	return func() {
		sto.mu.Lock()
		defer sto.mu.Unlock()

		subs := make([]*struct{ Subscriber }, 0, len(sto.subs)-1)
		for _, q := range sto.subs {
			if p != q {
				subs = append(subs, q)
			}
		}
		sto.subs = subs
	}
}

func (sto *Store) updateReducers(updaters ...func(*storeReducers)) *Store {
	sto.reducers.Lock()
	defer sto.reducers.Unlock()

	sto.reducers.storeReducers = sto.reducers.Copy(updaters...)
	return sto
}

// Before adds reducers to the list of reducers
// they're are called before normal (Store.Use) reducers
func (sto *Store) Before(reducers ...Reducer) *Store {
	return sto.updateReducers(func(sr *storeReducers) {
		sr.before = sr.before.Add(reducers...)
	})
}

// Use adds reducers to the list of reducers
// they're called after reducers added with Store.Before
// and before reducers added with Store.After
func (sto *Store) Use(reducers ...Reducer) *Store {
	return sto.updateReducers(func(sr *storeReducers) {
		sr.use = sr.use.Add(reducers...)
	})
}

// After adds reducers to the list of reducers
// they're are called after normal (Store.Use) reducers
func (sto *Store) After(reducers ...Reducer) *Store {
	return sto.updateReducers(func(sr *storeReducers) {
		sr.after = sr.after.Add(reducers...)
	})
}

// SetBaseConfig sets the EditorConfig on which State.Config is based
//
// this method is made available for use by editor/client integration
// normal users should use State.SetConfig instead
func (sto *Store) SetBaseConfig(cfg EditorConfig) *Store {
	sto.mu.Lock()
	defer sto.mu.Unlock()

	sto.cfg = cfg
	return sto
}

// Begin starts a new task and returns its ticket
func (sto *Store) Begin(t Task) *TaskTicket {
	return sto.tasks.Begin(t)
}

func (sto *Store) initCache(v *View) {
	cc := &sto.cache
	cc.Lock()
	defer cc.Unlock()

	if cc.vHash == v.Hash && cc.vName == v.Name {
		return
	}

	sto.KVMap.Clear()
	cc.vHash = v.Hash
	cc.vName = v.Name
}
