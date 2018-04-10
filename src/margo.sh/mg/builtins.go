package mg

import (
	"bytes"
	"fmt"
	"margo.sh/mgutil"
	"os"
	"sort"
	"strings"
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
	rc := bx.RunCmd
	if rc.Name == ".exec" {
		if len(rc.Args) == 0 {
			return bx.Done(nil)
		}
		rc.Name = rc.Args[0]
		rc.Args = rc.Args[1:]
	}

	p, err := starCmd(bx.Ctx, rc)
	if err != nil {
		a := append([]string{rc.Name}, rc.Args...)
		for i, s := range a {
			a[i] = mgutil.QuoteCmdArg(s)
		}
		return bx.Errorf("cannot exec `%s`: %s", strings.Join(a, " "), err)
	}

	go p.Wait()

	return bx.State
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

	return bx.Done(buf.Bytes())
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
	return bx.Done(buf.Bytes())
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
}

func (bx *BultinCmdCtx) Done(output []byte) *State {
	bx.Store.Dispatch(CmdOutput{Fd: bx.Fd, Output: output, Close: true})
	return bx.State
}

type BultinCmd struct {
	Name string
	Desc string
	Run  func(*BultinCmdCtx) *State
}
