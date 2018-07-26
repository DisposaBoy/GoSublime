package mg_test

import (
	"bytes"
	"io"
	"margo.sh/mg"
	"margo.sh/mgutil"
	"os"
	"strings"
	"testing"
)

func TestBuiltinCmdList_Lookup(t *testing.T) {
	t.Parallel()
	exec := func() mg.BuiltinCmd {
		r, _ := mg.Builtins.Commands().Lookup(".exec")
		return r
	}()
	item := mg.BuiltinCmd{
		Name: "this name",
		Desc: "description",
		Run:  func(*mg.CmdCtx) *mg.State { return nil },
	}
	tcs := []struct {
		name      string
		bcl       mg.BuiltinCmdList
		input     string
		wantCmd   mg.BuiltinCmd
		wantFound bool
	}{
		{"empty cmd list", mg.BuiltinCmdList{}, "nothing to find", exec, false},
		{"not found", mg.BuiltinCmdList{item}, "not found", exec, false},
		{"found", mg.BuiltinCmdList{item}, item.Name, item, true},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			gotCmd, gotFound := tc.bcl.Lookup(tc.input)
			// there is no way to compare functions, therefore we just check the names.
			if gotCmd.Name != tc.wantCmd.Name {
				t.Errorf("Lookup(): gotCmd = (%v); want (%v)", gotCmd, tc.wantCmd)
			}
			if gotFound != tc.wantFound {
				t.Errorf("Lookup(): gotFound = (%v); want (%v)", gotFound, tc.wantFound)
			}
		})
	}
}

// tests when the Args is empty, it should pick up the available BuiltinCmd(s).
func TestTypeCmdEmptyArgs(t *testing.T) {
	t.Parallel()
	item1 := mg.BuiltinCmd{Name: "this name", Desc: "this description"}
	item2 := mg.BuiltinCmd{Name: "another one", Desc: "should appear too"}
	buf := new(bytes.Buffer)
	input := &mg.CmdCtx{
		Ctx: &mg.Ctx{
			State: &mg.State{
				BuiltinCmds: mg.BuiltinCmdList{item1, item2},
			},
		},
		Output: &mgutil.IOWrapper{
			Writer: buf,
		},
	}

	if got := mg.Builtins.TypeCmd(input); got != input.State {
		t.Errorf("TypeCmd() = %v, want %v", got, input.State)
	}
	out := buf.String()
	for _, item := range []mg.BuiltinCmd{item1, item2} {
		if !strings.Contains(out, item.Name) {
			t.Errorf("buf.String() = (%s); want (%s) in it", out, item.Name)
		}
		if !strings.Contains(out, item.Desc) {
			t.Errorf("buf.String() = (%s); want (%s) in it", out, item.Desc)
		}
	}
}

func setupBuiltinCmdCtx(cmds mg.BuiltinCmdList, args []string, envMap mg.EnvMap, buf io.Writer) (*mg.CmdCtx, func()) {
	ctx := mg.NewTestingCtx(nil)
	ctx.State = ctx.AddBuiltinCmds(cmds...)
	ctx.Env = envMap
	rc := mg.RunCmd{Args: args}

	cmd := &mg.CmdCtx{
		Ctx:    ctx,
		RunCmd: rc,
		Output: &mgutil.IOWrapper{
			Writer: buf,
		},
	}
	return cmd, ctx.Cancel
}

// tests when command is found, it should choose it.
func TestTypeCmdLookupCmd(t *testing.T) {
	t.Parallel()
	item1 := mg.BuiltinCmd{Name: "this name", Desc: "this description"}
	item2 := mg.BuiltinCmd{Name: "another one", Desc: "should not appear"}
	buf := new(bytes.Buffer)
	input, cleanup := setupBuiltinCmdCtx(mg.BuiltinCmdList{item1, item2}, []string{item2.Name}, nil, buf)
	defer cleanup()

	if got := mg.Builtins.TypeCmd(input); got != input.State {
		t.Errorf("TypeCmd() = %v, want %v", got, input.State)
	}
	out := buf.String()
	if strings.Contains(out, item1.Name) {
		t.Errorf("buf.String() = (%s); didn't expect (%s) in it", out, item1.Name)
	}
	if strings.Contains(out, item1.Name) {
		t.Errorf("buf.String() = (%s); didn't expect (%s) in it", out, item1.Name)
	}
	if !strings.Contains(out, item2.Name) {
		t.Errorf("buf.String() = (%s); want (%s) in it", out, item2.Name)
	}
	if !strings.Contains(out, item2.Name) {
		t.Errorf("buf.String() = (%s); want (%s) in it", out, item2.Name)
	}
}

type envPair struct {
	key   string
	value string
}

func setupEnvCmd(t *testing.T) ([]envPair, []mg.BuiltinCmd, func()) {
	envs := []envPair{
		{"thiskey", "the value"},
		{"anotherkey", "another value"},
	}
	items := make([]mg.BuiltinCmd, len(envs))
	for i, e := range envs {
		if err := os.Setenv(e.key, e.value); err != nil {
			t.Fatalf("cannot set environment values: %v", err)
		}
		items[i] = mg.BuiltinCmd{Name: e.key, Desc: "doesn't matter"}
	}
	cleanup := func() {
		for _, e := range envs {
			os.Unsetenv(e.key)
		}
	}
	return envs, items, cleanup
}

func TestEnvCmd(t *testing.T) {
	t.Parallel()
	envs, items, cleanup := setupEnvCmd(t)
	defer cleanup()

	tcs := []struct {
		name     string
		cmds     mg.BuiltinCmdList
		args     []string
		envs     mg.EnvMap
		wantEnvs []envPair
	}{
		{"no cmd no env", mg.BuiltinCmdList{}, []string{}, nil, nil},
		{
			"no cmd with env",
			mg.BuiltinCmdList{},
			[]string{},
			mg.EnvMap{"tAlbt": "gRuVbi", "wILHI": "XOmsUdw"},
			[]envPair{{"tAlbt", "gRuVbi"}, {"wILHI", "XOmsUdw"}},
		},
		{
			"one env pair", mg.BuiltinCmdList{items[0]},
			[]string{items[0].Name},
			nil,
			[]envPair{envs[0]},
		},
		{
			"multiple env pairs",
			mg.BuiltinCmdList{items[0], items[1]},
			[]string{items[0].Name, items[1].Name},
			nil,
			[]envPair{envs[0], envs[1]},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			input, cleanup := setupBuiltinCmdCtx(tc.cmds, tc.args, tc.envs, buf)
			defer cleanup()
			if got := mg.Builtins.EnvCmd(input); got == nil {
				t.Error("EnvCmd() = (nil); want (*State)")
			}
			out := buf.String()
			for _, e := range tc.wantEnvs {
				if !strings.Contains(out, e.key) {
					t.Errorf("buf.String() = (%s); want (%s) in it", out, e.key)
				}
				if !strings.Contains(out, e.value) {
					t.Errorf("buf.String() = (%s); want (%s) in it", out, e.value)
				}
			}
		})
	}
}

func TestBuiltinCmdsReduce(t *testing.T) {
	t.Parallel()
	isIn := func(cmd mg.BuiltinCmd, haystack mg.BuiltinCmdList) bool {
		for _, h := range haystack {
			if h.Name == cmd.Name && h.Desc == cmd.Desc {
				return true
			}
		}
		return false
	}

	item := mg.BuiltinCmd{Name: "qNgEYow", Desc: "YKjYxqMnt"}
	ctx := &mg.Ctx{
		State: &mg.State{
			BuiltinCmds: mg.BuiltinCmdList{item},
		},
	}

	bc := mg.Builtins
	state := bc.Reduce(ctx)
	if state == nil {
		t.Fatal("bc.Reduce() = nil, want *State")
	}
	for _, cmd := range bc.Commands() {
		if isIn(cmd, state.BuiltinCmds) {
			t.Errorf("didn't want %v in %v", cmd, bc.Commands())
		}
	}
	if !isIn(item, state.BuiltinCmds) {
		t.Errorf("want %v in %v", item, bc.Commands())
	}

	ctx.Action = mg.RunCmd{}
	state = bc.Reduce(ctx)
	if state == nil {
		t.Fatal("bc.Reduce() = nil, want *State")
	}
	for _, cmd := range bc.Commands() {
		if !isIn(cmd, state.BuiltinCmds) {
			t.Errorf("want %v in %v", cmd, bc.Commands())
		}
	}
	if !isIn(item, state.BuiltinCmds) {
		t.Errorf("want %v in %v", item, bc.Commands())
	}
}
