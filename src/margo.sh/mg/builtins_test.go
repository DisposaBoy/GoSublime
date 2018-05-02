package mg_test

import (
	"bytes"
	"io"
	"margo.sh/mg"
	"os"
	"strings"
	"testing"
)

func TestBultinCmdList_Lookup(t *testing.T) {
	t.Parallel()
	exec := func() mg.BultinCmd {
		r, _ := mg.Builtins.Commands().Lookup(".exec")
		return r
	}()
	item := mg.BultinCmd{
		Name: "this name",
		Desc: "description",
		Run:  func(*mg.BultinCmdCtx) *mg.State { return nil },
	}
	tcs := []struct {
		name      string
		bcl       mg.BultinCmdList
		input     string
		wantCmd   mg.BultinCmd
		wantFound bool
	}{
		{"empty cmd list", mg.BultinCmdList{}, "nothing to find", exec, false},
		{"not found", mg.BultinCmdList{item}, "not found", exec, false},
		{"found", mg.BultinCmdList{item}, item.Name, item, true},
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
	item1 := mg.BultinCmd{Name: "this name", Desc: "this description"}
	item2 := mg.BultinCmd{Name: "another one", Desc: "should appear too"}
	buf := new(bytes.Buffer)
	input := &mg.BultinCmdCtx{
		Ctx: &mg.Ctx{
			State: &mg.State{
				BuiltinCmds: mg.BultinCmdList{item1, item2},
			},
		},
		Output: &mg.CmdOutputWriter{
			Writer:   buf,
			Dispatch: nil,
		},
	}

	if got := mg.Builtins.TypeCmd(input); got != input.State {
		t.Errorf("TypeCmd() = %v, want %v", got, input.State)
	}
	out := buf.String()
	for _, item := range []mg.BultinCmd{item1, item2} {
		if !strings.Contains(out, item.Name) {
			t.Errorf("buf.String() = (%s); want (%s) in it", out, item.Name)
		}
		if !strings.Contains(out, item.Desc) {
			t.Errorf("buf.String() = (%s); want (%s) in it", out, item.Desc)
		}
	}
}

func setupBultinCmdCtx(cmds mg.BultinCmdList, args []string, envMap mg.EnvMap, buf io.Writer) (*mg.BultinCmdCtx, func()) {
	ctx := mg.NewTestingCtx(nil)
	ctx.State = ctx.AddBuiltinCmds(cmds...)
	ctx.Env = envMap
	rc := mg.RunCmd{Args: args}

	cmd := mg.NewBultinCmdCtx(ctx, rc)
	cmd.Output = &mg.CmdOutputWriter{
		Writer:   buf,
		Dispatch: nil,
	}
	return cmd, ctx.Cancel
}

// tests when command is found, it should choose it.
func TestTypeCmdLookupCmd(t *testing.T) {
	t.Parallel()
	item1 := mg.BultinCmd{Name: "this name", Desc: "this description"}
	item2 := mg.BultinCmd{Name: "another one", Desc: "should not appear"}
	buf := new(bytes.Buffer)
	input, cleanup := setupBultinCmdCtx(mg.BultinCmdList{item1, item2}, []string{item2.Name}, nil, buf)
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

func setupEnvCmd(t *testing.T) ([]envPair, []mg.BultinCmd, func()) {
	envs := []envPair{
		{"thiskey", "the value"},
		{"anotherkey", "another value"},
	}
	items := make([]mg.BultinCmd, len(envs))
	for i, e := range envs {
		if err := os.Setenv(e.key, e.value); err != nil {
			t.Fatalf("cannot set environment values: %v", err)
		}
		items[i] = mg.BultinCmd{Name: e.key, Desc: "doesn't matter"}
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
		cmds     mg.BultinCmdList
		args     []string
		envs     mg.EnvMap
		wantEnvs []envPair
	}{
		{"no cmd no env", mg.BultinCmdList{}, []string{}, nil, nil},
		{
			"no cmd with env",
			mg.BultinCmdList{},
			[]string{},
			mg.EnvMap{"tAlbt": "gRuVbi", "wILHI": "XOmsUdw"},
			[]envPair{{"tAlbt", "gRuVbi"}, {"wILHI", "XOmsUdw"}},
		},
		{
			"one env pair", mg.BultinCmdList{items[0]},
			[]string{items[0].Name},
			nil,
			[]envPair{envs[0]},
		},
		{
			"multiple env pairs",
			mg.BultinCmdList{items[0], items[1]},
			[]string{items[0].Name, items[1].Name},
			nil,
			[]envPair{envs[0], envs[1]},
		},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			input, cleanup := setupBultinCmdCtx(tc.cmds, tc.args, tc.envs, buf)
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
	isIn := func(cmd mg.BultinCmd, haystack mg.BultinCmdList) bool {
		for _, h := range haystack {
			if h.Name == cmd.Name && h.Desc == cmd.Desc {
				return true
			}
		}
		return false
	}

	item := mg.BultinCmd{Name: "qNgEYow", Desc: "YKjYxqMnt"}
	ctx := &mg.Ctx{
		State: &mg.State{
			BuiltinCmds: mg.BultinCmdList{item},
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
