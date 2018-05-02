package mg

import (
	"margo.sh/misc/pf"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var _ Dispatcher = (&Store{}).Dispatch

type Dispatcher func(Action)

type Subscriber func(*Ctx)

type dispatchHandler func()

type storeReducers struct {
	before reducerList
	use    reducerList
	after  reducerList
}

func (sr storeReducers) Reduce(mx *Ctx) *Ctx {
	mx.Profile.Do("Before", func() {
		mx = sr.before.callReducers(mx)
	})
	mx.Profile.Do("Use", func() {
		mx = sr.use.callReducers(mx)
	})
	mx.Profile.Do("After", func() {
		mx = sr.after.callReducers(mx)
	})
	return mx
}

func (sr storeReducers) Copy(updaters ...func(*storeReducers)) storeReducers {
	for _, f := range updaters {
		f(&sr)
	}
	return sr
}

type Store struct {
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

	mounted map[Reducer]bool
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

func (sto *Store) Dispatch(act Action) {
	sto.dsp.lo <- func() { sto.handleAct(act, nil) }
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
	sto.handleAct(Started{}, nil)
	sto.ag.Log.Println("started")

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

func (sto *Store) handle(h func() *Ctx, p *pf.Profile) {
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

func (sto *Store) handleAct(act Action, p *pf.Profile) {
	if p == nil {
		p = pf.NewProfile("")
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
		act, err := sto.ag.createAction(ra, sto.ag.handle)
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
		v.initSrcPos()
	}
	if len(props.Env) != 0 {
		mx.Env = props.Env
	}
	mx.Env = sto.autoSwitchInternalGOPATH(mx.View, mx.Env)
	return mx, acts
}

// autoSwitchInternalGOPATH automatically changes env[GOPATH] to the internal GOPATH
// if v.Filename is a child of one of the internal GOPATH directories
func (sto *Store) autoSwitchInternalGOPATH(v *View, env EnvMap) EnvMap {
	osGopath := os.Getenv("GOPATH")
	fn := v.Filename()
	for _, dir := range strings.Split(osGopath, string(filepath.ListSeparator)) {
		if IsParentDir(dir, fn) {
			return env.Add("GOPATH", osGopath)
		}
	}
	return env
}

func (sto *Store) State() *State {
	sto.mu.Lock()
	defer sto.mu.Unlock()

	return sto.state
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
	sto.state = newState(sto)
	sto.tasks = &taskTracker{}
	sto.After(sto.tasks)

	// 640 slots ought to be enough for anybody
	sto.dsp.lo = make(chan dispatchHandler, 640)
	sto.dsp.hi = make(chan dispatchHandler, 640)

	sto.mounted = map[Reducer]bool{}

	return sto
}

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

func (sto *Store) EditorConfig(cfg EditorConfig) *Store {
	sto.mu.Lock()
	defer sto.mu.Unlock()

	sto.cfg = cfg
	return sto
}

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
