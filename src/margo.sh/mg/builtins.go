package mg

import (
	"bytes"
	"fmt"
	"os"
	"sort"
)

var (
	Builtins = builtins{}
)

type BultinCmdList []BultinCmd

// Lookup looks up the builtin command `name` in the list.
// If the command is not found, it returns `Builtins.Commands().Lookup(".exec")`.
// In either case, `found` indicates whether or not `name` was actually found.
func (bcl BultinCmdList) Lookup(name string) (cmd BultinCmd, found bool) {
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
func (b builtins) ExecCmd(bx *BultinCmdCtx) *State {
	go b.execCmd(bx)
	return bx.State
}

func (b builtins) execCmd(bx *BultinCmdCtx) {
	defer bx.Output.Close()

	if bx.Name == ".exec" {
		if len(bx.Args) == 0 {
			return
		}
		bx = bx.Copy(func(bx *BultinCmdCtx) {
			bx.Name = bx.Args[0]
			bx.Args = bx.Args[1:]
		})
	}

	bx.RunProc()
}

// TypeCmd tries to find the bx.Args in commands, and writes the description of
// the commands into provided buffer. If the Args is empty, it uses all
// available commands.
func (b builtins) TypeCmd(bx *BultinCmdCtx) *State {
	cmds := bx.BuiltinCmds
	names := bx.Args
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

	bx.Output.Close(buf.Bytes())
	return bx.State
}

// EnvCmd finds all environment variables corresponding to bx.Args into the
// bx.Output buffer.
func (b builtins) EnvCmd(bx *BultinCmdCtx) *State {
	buf := &bytes.Buffer{}
	names := bx.Args
	if len(names) == 0 {
		names = make([]string, 0, len(bx.Env))
		for k, _ := range bx.Env {
			names = append(names, k)
		}
		sort.Strings(names)
	}
	for _, k := range names {
		v := bx.Env.Get(k, os.Getenv(k))
		fmt.Fprintf(buf, "%s=%s\n", k, v)
	}
	bx.Output.Close(buf.Bytes())
	return bx.State
}

// Commands returns a list of predefined commands.
func (b builtins) Commands() BultinCmdList {
	return []BultinCmd{
		BultinCmd{Name: ".env", Desc: "List env vars", Run: b.EnvCmd},
		BultinCmd{Name: ".exec", Desc: "Run a command through os/exec", Run: b.ExecCmd},
		BultinCmd{Name: ".type", Desc: "Lists all builtins or which builtin handles a command", Run: b.TypeCmd},
	}
}

// Reduce adds the list of predefined builtins for the RunCmd.
func (bc builtins) Reduce(mx *Ctx) *State {
	if _, ok := mx.Action.(RunCmd); ok {
		return mx.State.AddBuiltinCmds(bc.Commands()...)
	}
	return mx.State
}

type BultinCmdCtx struct {
	*Ctx
	RunCmd
	Output *CmdOutputWriter
}

func NewBultinCmdCtx(mx *Ctx, rc RunCmd) *BultinCmdCtx {
	return &BultinCmdCtx{
		Ctx:    mx,
		RunCmd: rc,
		Output: &CmdOutputWriter{Fd: rc.Fd, Dispatch: mx.Store.Dispatch},
	}
}

func (bx *BultinCmdCtx) update(updaters ...func(*BultinCmdCtx)) *BultinCmdCtx {
	for _, f := range updaters {
		f(bx)
	}
	return bx
}

func (bx *BultinCmdCtx) Copy(updaters ...func(*BultinCmdCtx)) *BultinCmdCtx {
	x := *bx
	return x.update(updaters...)
}

func (bx *BultinCmdCtx) RunProc() {
	p, err := bx.StartProc()
	if err == nil {
		err = p.Wait()
	}
	if err != nil {
		fmt.Fprintf(bx.Output, "`%s` exited: %s\n", p.Title, err)
	}
}

// StartProc creates a new Proc and starts the underlying process.
// It always returns an initialised Proc.
func (bx *BultinCmdCtx) StartProc() (*Proc, error) {
	p := newProc(bx)
	return p, p.start()
}

type BultinCmd struct {
	Name string
	Desc string
	Run  func(*BultinCmdCtx) *State
}
