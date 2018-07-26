package mg

import (
	"testing"
)

func TestCmdSupport_Reduce_noCalls(t *testing.T) {
	type unknown struct{ ActionType }
	cs := &cmdSupport{}
	ctx := NewTestingCtx(nil)
	defer ctx.Cancel()

	if state := cs.Reduce(ctx); state != ctx.State {
		t.Errorf("cmdSupport.Reduce() = %v, want %v", state, ctx.State)
	}

	ctx.Action = new(unknown)
	if state := cs.Reduce(ctx); state != ctx.State {
		t.Errorf("cmdSupport.Reduce() = %v, want %v", state, ctx.State)
	}
}

func TestCmdSupport_Reduce_withRunCmd(t *testing.T) {
	var called bool
	cs := &cmdSupport{}
	ctx := NewTestingCtx(RunCmd{
		Fd:   "rHX23",
		Name: ".mytest",
	})
	defer ctx.Cancel()

	ctx.State = ctx.AddBuiltinCmds(BuiltinCmd{
		Name: ".mytest",
		Run: func(cx *CmdCtx) *State {
			called = true
			return cx.State
		},
	})

	if state := cs.Reduce(ctx); state != ctx.State {
		t.Errorf("cmdSupport.Reduce() = %v, want %v", state, ctx.State)
	}
	if !called {
		t.Errorf("cs.Reduce(%v): cs.runCmd() wasn't called", ctx)
	}
}

func TestCmdSupport_Reduce_withCmdOutput(t *testing.T) {
	var called bool
	fd := "CIlZ7zBWHIAL"
	cs := &clientActionSupport{}
	ctx := NewTestingCtx(nil)
	defer ctx.Cancel()

	ctx.Action = CmdOutput{
		Fd: fd,
	}

	state := cs.Reduce(ctx)
	for _, c := range state.clientActions {
		if d, ok := c.Data.(CmdOutput); ok && d.Fd == fd {
			called = true
			break
		}
	}
	if !called {
		t.Errorf("cs.Reduce(%v): cs.cmdOutput() wasn't called", ctx)
	}
}
