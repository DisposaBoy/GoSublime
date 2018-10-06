package mg

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"sync"
)

var (
	// Builtins is the set of pre-defined builtin commands
	Builtins = &builtins{}
)

// BuiltinCmdList is a list of BuiltinCmds
type BuiltinCmdList []BuiltinCmd

// Lookup looks up the builtin command `name` in the list.
// If the command is not found, it returns `Builtins.Commands().Lookup(".exec")`.
// In either case, `found` indicates whether or not `name` was actually found.
func (bcl BuiltinCmdList) Lookup(name string) (cmd BuiltinCmd, found bool) {
	for _, c := range bcl {
		if c.Name == name {
			return c, true
		}
	}
	for _, c := range Builtins.Commands() {
		if c.Name == ".exec" {
			return c, false
		}
	}
	panic("internal error: the `.exec` BuiltinCmd is not defined")
}

// Filter returns a copy of the list consisting only
// of commands for which filter returns true
func (bcl BuiltinCmdList) Filter(filter func(BuiltinCmd) bool) BuiltinCmdList {
	cmds := BuiltinCmdList{}
	for _, l := range []BuiltinCmdList{bcl, Builtins.Commands()} {
		for _, c := range l {
			if filter(c) {
				cmds = append(cmds, c)
			}
		}
	}
	return cmds
}

// BuiltinCmds implements various builtin commands.
type builtins struct{ ReducerType }

// ExecCmd implements the `.exec` builtin.
func (bc builtins) ExecCmd(cx *CmdCtx) *State {
	go bc.execCmd(cx)
	return cx.State
}

func (bc builtins) nopRun(cx *CmdCtx) *State {
	defer cx.Output.Close()
	return cx.State
}

func (bc builtins) execCmd(cx *CmdCtx) {
	defer cx.Output.Close()

	if cx.Name == ".exec" {
		if len(cx.Args) == 0 {
			return
		}
		cx = cx.Copy(func(cx *CmdCtx) {
			cx.Name = cx.Args[0]
			cx.Args = cx.Args[1:]
		})
	}

	cx.RunProc()
}

// TypeCmd tries to find the cx.Args in commands, and writes the description of
// the commands into provided buffer. If the Args is empty, it uses all
// available commands.
func (bc builtins) TypeCmd(cx *CmdCtx) *State {
	defer cx.Output.Close()

	cmds := cx.BuiltinCmds
	names := cx.Args
	if len(names) == 0 {
		names = make([]string, len(cmds))
		for i, c := range cmds {
			names[i] = c.Name
		}
	}

	buf := &bytes.Buffer{}
	for _, name := range names {
		c, _ := cmds.Lookup(name)
		fmt.Fprintf(buf, "%s: builtin: %s, desc: %s\n", name, c.Name, c.Desc)
	}

	cx.Output.Write(buf.Bytes())
	return cx.State
}

// EnvCmd finds all environment variables corresponding to cx.Args into the
// cx.Output buffer.
func (bc builtins) EnvCmd(cx *CmdCtx) *State {
	defer cx.Output.Close()

	buf := &bytes.Buffer{}
	names := cx.Args
	if len(names) == 0 {
		names = make([]string, 0, len(cx.Env))
		for k, _ := range cx.Env {
			names = append(names, k)
		}
		sort.Strings(names)
	}
	for _, k := range names {
		v := cx.Env.Get(k, os.Getenv(k))
		fmt.Fprintf(buf, "%s=%s\n", k, v)
	}
	cx.Output.Write(buf.Bytes())
	return cx.State
}

// Commands returns a list of predefined commands.
func (bc builtins) Commands() BuiltinCmdList {
	return []BuiltinCmd{
		BuiltinCmd{Name: ".env", Desc: "List env vars", Run: bc.EnvCmd},
		BuiltinCmd{Name: ".exec", Desc: "Run a command through os/exec", Run: bc.ExecCmd},
		BuiltinCmd{Name: ".type", Desc: "Lists all builtins or which builtin handles a command", Run: bc.TypeCmd},

		// virtual commands implemented by other reducers
		// these are fallbacks, so no error is reported for the missing command
		BuiltinCmd{Name: RcActuate, Desc: "Trigger a mouse-like action at the cursor e.g. goto.definition", Run: bc.nopRun},
	}
}

// Reduce adds the list of predefined builtins for the RunCmd.
func (bc builtins) Reduce(mx *Ctx) *State {
	if _, ok := mx.Action.(RunCmd); ok {
		return mx.State.AddBuiltinCmds(bc.Commands()...)
	}
	return mx.State
}

// CmdCtx holds details about a command execution
type CmdCtx struct {
	// Ctx is the underlying Ctx for the current reduction
	*Ctx

	// RunCmd is the action that was dispatched
	RunCmd

	// Output is the `stdout` of the command.
	// Commands must close it when are done.
	Output OutputStream
}

func (cx *CmdCtx) update(updaters ...func(*CmdCtx)) *CmdCtx {
	for _, f := range updaters {
		f(cx)
	}
	return cx
}

// Copy returns a shallow copy of the CmdCtx
func (cx *CmdCtx) Copy(updaters ...func(*CmdCtx)) *CmdCtx {
	x := *cx
	return x.update(updaters...)
}

// WithCmd returns a copy of cx RunCmd updated with Name name and Args args
func (cx *CmdCtx) WithCmd(name string, args ...string) *CmdCtx {
	return cx.Copy(func(cx *CmdCtx) {
		rc := cx.RunCmd
		rc.Name = name
		rc.Args = args
		cx.RunCmd = rc
	})
}

// Run runs the list of builtin commands with name CmtCtx.RunCmd.Name.
// If no commands exist with that name, it calls Builtins.ExecCmd instead.
func (cx *CmdCtx) Run() *State {
	cmds := cx.BuiltinCmds.Filter(func(c BuiltinCmd) bool { return c.Name == cx.Name })
	switch len(cmds) {
	case 0:
		return Builtins.ExecCmd(cx)
	case 1:
		return cmds[0].Run(cx)
	}

	stream := cx.Output
	defer stream.Close()
	wg := &sync.WaitGroup{}
	defer wg.Wait()

	st := cx.State
	for _, c := range cmds {
		cx = cx.Copy(func(x *CmdCtx) {
			x.Ctx = x.Ctx.SetState(st)
			x.Output = newOutputStreamRef(wg, stream)
		})
		st = c.Run(cx)
	}
	return st
}

// RunProc is a convenience function that:
// * calls StartProc()
// * waits for the process to complete
// * and logs any returned error to CmdCtx.Output
func (cx *CmdCtx) RunProc() {
	p, err := cx.StartProc()
	if err == nil {
		err = p.Wait()
	}
	if err != nil {
		fmt.Fprintf(cx.Output, "`%s` exited: %s\n", p.Title, err)
	}
}

// StartProc creates a new Proc and starts the underlying process.
// It always returns an initialised Proc.
func (cx *CmdCtx) StartProc() (*Proc, error) {
	p := newProc(cx)
	return p, p.start()
}

// BuiltinCmdRunFunc is the BuiltinCmd.Run function
//
// Where possible, implementations should prefer to do real work in a goroutine.
type BuiltinCmdRunFunc func(*CmdCtx) *State

// BuiltinCmd describes a builtin command
type BuiltinCmd struct {
	// Name is the name of the name.
	Name string

	// Desc is a description of what the command does
	Desc string

	// Run is called to carry out the operation of the command
	Run BuiltinCmdRunFunc
}
