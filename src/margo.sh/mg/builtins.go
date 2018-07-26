package mg

import (
	"bytes"
	"fmt"
	"os"
	"sort"
)

var (
	Builtins = &builtins{}
)

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

// BuiltinCmds implements various builtin commands.
type builtins struct{ ReducerType }

// ExecCmd implements the `.exec` builtin.
func (b builtins) ExecCmd(cx *CmdCtx) *State {
	go b.execCmd(cx)
	return cx.State
}

func (b builtins) execCmd(cx *CmdCtx) {
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
func (b builtins) TypeCmd(cx *CmdCtx) *State {
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
func (b builtins) EnvCmd(cx *CmdCtx) *State {
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
func (b builtins) Commands() BuiltinCmdList {
	return []BuiltinCmd{
		BuiltinCmd{Name: ".env", Desc: "List env vars", Run: b.EnvCmd},
		BuiltinCmd{Name: ".exec", Desc: "Run a command through os/exec", Run: b.ExecCmd},
		BuiltinCmd{Name: ".type", Desc: "Lists all builtins or which builtin handles a command", Run: b.TypeCmd},
	}
}

// Reduce adds the list of predefined builtins for the RunCmd.
func (bc builtins) Reduce(mx *Ctx) *State {
	if _, ok := mx.Action.(RunCmd); ok {
		return mx.State.AddBuiltinCmds(bc.Commands()...)
	}
	return mx.State
}

type CmdCtx struct {
	*Ctx
	RunCmd
	Output OutputStream
}

func (cx *CmdCtx) update(updaters ...func(*CmdCtx)) *CmdCtx {
	for _, f := range updaters {
		f(cx)
	}
	return cx
}

func (cx *CmdCtx) Copy(updaters ...func(*CmdCtx)) *CmdCtx {
	x := *cx
	return x.update(updaters...)
}

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

type BuiltinCmd struct {
	Name string
	Desc string
	Run  BuiltinCmdRunFunc
}

// ExecRunFunc returns a BuiltinCMd.Run function that wraps Builtins.ExecCmd
// It sets the received CmdCtx.RunCmd's Name and Args fields
func ExecRunFunc(name string, args ...string) BuiltinCmdRunFunc {
	return func(cx *CmdCtx) *State {
		x := *cx
		x.RunCmd.Name = name
		x.RunCmd.Args = args
		return Builtins.ExecCmd(&x)
	}
}
