package mg

import (
	"bytes"
	"fmt"
	"margo.sh/mgutil"
	"os"
	"sort"
)

var (
	Builtins = BuiltinCmds{}
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

type BuiltinCmds struct{}

func (bc BuiltinCmds) ExecCmd(bx *BultinCmdCtx) *State {
	go bc.execCmd(bx)
	return bx.State
}

func (bc BuiltinCmds) execCmd(bx *BultinCmdCtx) {
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

func (bc BuiltinCmds) TypeCmd(bx *BultinCmdCtx) *State {
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

func (bc BuiltinCmds) EnvCmd(bx *BultinCmdCtx) *State {
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

func (bc BuiltinCmds) Commands() BultinCmdList {
	return []BultinCmd{
		BultinCmd{Name: ".env", Desc: "List env vars", Run: bc.EnvCmd},
		BultinCmd{Name: ".exec", Desc: "Run a command through os/exec", Run: bc.ExecCmd},
		BultinCmd{Name: ".type", Desc: "Lists all builtins or which builtin handles a command", Run: bc.TypeCmd},
	}
}

func (bc BuiltinCmds) Reduce(mx *Ctx) *State {
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
		fmt.Fprintf(bx.Output, "`%s` exited: %s\n", mgutil.QuoteCmd(bx.Name, bx.Args...), err)
	}
}

func (bx *BultinCmdCtx) StartProc() (*Proc, error) {
	p := newProc(bx)
	return p, p.start()
}

type BultinCmd struct {
	Name string
	Desc string
	Run  func(*BultinCmdCtx) *State
}
