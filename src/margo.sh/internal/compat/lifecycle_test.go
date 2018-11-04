package compat

import (
	"margo.sh/mg"
	"margo.sh/mgutil"
	"runtime"
	"strings"
	"testing"
)

type lifecycleState struct {
	names []string
}

func (ls *lifecycleState) called() {
	pc, _, _, _ := runtime.Caller(1)
	f := runtime.FuncForPC(pc)
	l := strings.Split(f.Name(), ".")
	target := l[len(l)-1]
	for i, s := range ls.names {
		if s == target {
			ls.names = append(ls.names[:i], ls.names[i+1:]...)
			break
		}
	}
}

func (ls *lifecycleState) uncalled() string {
	return strings.Join(ls.names, ", ")
}

func (ls *lifecycleState) test(t *testing.T, r mg.Reducer) {
	t.Helper()
	ag := newAgent(`{}`)
	ag.Store.Use(r)
	ag.Run()
	if s := ls.uncalled(); s != "" {
		t.Fatalf("reduction failed to call: %s", s)
	}
}

type legacyLifecycleEmbedded struct {
	mg.Reducer
}

type lifecycle struct {
	mg.ReducerType
	*lifecycleState
}

func (l *lifecycle) Reduce(mx *mg.Ctx) *mg.State       { l.called(); return mx.State }
func (l *lifecycle) RInit(_ *mg.Ctx)                   { l.called() }
func (l *lifecycle) RConfig(_ *mg.Ctx) mg.EditorConfig { l.called(); return nil }
func (l *lifecycle) RCond(_ *mg.Ctx) bool              { l.called(); return true }
func (l *lifecycle) RMount(_ *mg.Ctx)                  { l.called() }
func (l *lifecycle) RUnmount(_ *mg.Ctx)                { l.called() }

type legacyLifecycle struct {
	mg.ReducerType
	*lifecycleState
}

func (l *legacyLifecycle) Reduce(mx *mg.Ctx) *mg.State             { l.called(); return mx.State }
func (l *legacyLifecycle) ReducerInit(_ *mg.Ctx)                   { l.called() }
func (l *legacyLifecycle) ReducerConfig(_ *mg.Ctx) mg.EditorConfig { l.called(); return nil }
func (l *legacyLifecycle) ReducerCond(_ *mg.Ctx) bool              { l.called(); return true }
func (l *legacyLifecycle) ReducerMount(_ *mg.Ctx)                  { l.called() }
func (l *legacyLifecycle) ReducerUnmount(_ *mg.Ctx)                { l.called() }

type lifecycleEmbedded struct {
	mg.Reducer
}

func TestLifecycleMethodCalls(t *testing.T) {
	names := func() []string {
		return []string{
			"Reduce", "RInit", "RConfig",
			"RCond", "RMount", "RUnmount",
		}
	}
	legacyNames := func() []string {
		return []string{
			"Reduce", "ReducerInit", "ReducerConfig",
			"ReducerCond", "ReducerMount", "ReducerUnmount",
		}
	}
	t.Run("Direct Calls", func(t *testing.T) {
		ls := &lifecycleState{names: names()}
		ls.test(t, &lifecycle{lifecycleState: ls})
	})
	t.Run("Embedded Calls", func(t *testing.T) {
		ls := &lifecycleState{names: names()}
		ls.test(t, &lifecycleEmbedded{&lifecycle{lifecycleState: ls}})
	})
	t.Run("Legacy Direct Calls", func(t *testing.T) {
		ls := &lifecycleState{names: legacyNames()}
		ls.test(t, &legacyLifecycle{lifecycleState: ls})
	})
	t.Run("Legacy Embedded Calls", func(t *testing.T) {
		ls := &lifecycleState{names: legacyNames()}
		ls.test(t, &lifecycleEmbedded{&legacyLifecycle{lifecycleState: ls}})
	})
}

func TestLifecycleMountedAndUnmounted(t *testing.T) {
	cond := false
	mount := false
	unmount := false
	r := &mg.RFunc{
		Cond: func(*mg.Ctx) bool {
			cond = true
			// we want to return false if the reducer mounted
			// this tests that RUnmount is called if RMount was called
			// even if RCond returns false at some point in the future
			//
			// at the time this test was written, the implementation was wrong
			// because if RCond returned falsed, no other method was called
			// so it would appear correct, but that was just a coincidence
			if mount {
				return false
			}
			return true
		},
		Mount:   func(*mg.Ctx) { mount = true },
		Unmount: func(*mg.Ctx) { unmount = true },
	}
	newAgent(`{}`, r).Run()
	if !cond {
		t.Fatal("Reducer.RCond was not called")
	}
	if !mount {
		t.Fatal("Reducer.RMount was not called")
	}
	if !unmount {
		t.Fatal("Reducer.RUnmount was not called")
	}
}

func TestLifecycleNotMounted(t *testing.T) {
	init := false
	config := false
	cond := false
	mount := false
	reduce := false
	unmount := false
	r := &mg.RFunc{
		Init:    func(*mg.Ctx) { init = true },
		Config:  func(*mg.Ctx) mg.EditorConfig { config = true; return nil },
		Cond:    func(*mg.Ctx) bool { cond = true; return false },
		Mount:   func(*mg.Ctx) { mount = true },
		Func:    func(mx *mg.Ctx) *mg.State { reduce = true; return mx.State },
		Unmount: func(*mg.Ctx) { unmount = true },
	}
	newAgent(`{}`, r).Run()
	if !init {
		t.Fatal("Reducer.RInit was not called")
	}
	if !config {
		t.Fatal("Reducer.RConfig was not called")
	}
	if !cond {
		t.Fatal("Reducer.RCond was not called")
	}
	if mount {
		t.Fatal("Reducer.RMount was called")
	}
	if reduce {
		t.Fatal("Reducer.Reduce was called")
	}
	if unmount {
		t.Fatal("Reducer.RUnmount was called")
	}
}

func TestLifecycleMounted(t *testing.T) {
	init := false
	config := false
	mount := false
	reduce := false
	unmount := false
	r := &mg.RFunc{
		Init:   func(*mg.Ctx) { init = true },
		Config: func(*mg.Ctx) mg.EditorConfig { config = true; return nil },
		// Cond is implicitly true
		Mount:   func(*mg.Ctx) { mount = true },
		Func:    func(mx *mg.Ctx) *mg.State { reduce = true; return mx.State },
		Unmount: func(*mg.Ctx) { unmount = true },
	}
	newAgent(`{}`, r).Run()
	if !init {
		t.Fatal("Reducer.RInit was not called")
	}
	if !config {
		t.Fatal("Reducer.RConfig was not called")
	}
	if !mount {
		t.Fatal("Reducer.RMount was not called")
	}
	if !reduce {
		t.Fatal("Reducer.Reduce was not called")
	}
	if !unmount {
		t.Fatal("Reducer.RUnmount was not called")
	}
}

func newAgent(stdinJSON string, use ...mg.Reducer) *mg.Agent {
	ag, _ := mg.NewAgent(mg.AgentConfig{
		Stdin:  &mgutil.IOWrapper{Reader: strings.NewReader(stdinJSON)},
		Stdout: &mgutil.IOWrapper{},
		Stderr: &mgutil.IOWrapper{},
	})
	ag.Store.Use(use...)
	return ag
}
