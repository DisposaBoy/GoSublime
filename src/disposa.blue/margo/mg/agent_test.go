package mg

import (
	"os"
	"strings"
	"testing"
)

// TestDefaults tries to verify some assumptions that are, or will be, made throughout the code-base
// the following should hold true regardless of what configuration is exposed in the future
// * the default codec should be json
// * logs should go to os.Stderr by default
// * IPC communication should be done on os.Stdin and os.Stdout by default
func TestDefaults(t *testing.T) {
	ag, err := NewAgent(AgentConfig{})
	if err != nil {
		t.Errorf("agent creation failed: %s", err)
		return
	}

	stdin := ag.stdin
	if w, ok := stdin.(*LockedReadCloser); ok {
		stdin = w.ReadCloser
	}
	stdout := ag.stdout
	if w, ok := stdout.(*LockedWriteCloser); ok {
		stdout = w.WriteCloser
	}
	stderr := ag.stderr
	if w, ok := stderr.(*LockedWriteCloser); ok {
		stderr = w.WriteCloser
	}

	cases := []struct {
		name   string
		expect interface{}
		got    interface{}
	}{
		{`DefaultCodec == json`, true, DefaultCodec == "json"},
		{`codecHandles[DefaultCodec] exists`, true, codecHandles[DefaultCodec] != nil},
		{`codecHandles[""] == codecHandles[DefaultCodec]`, true, codecHandles[""] == codecHandles[DefaultCodec]},
		{`default Agent.stdin`, os.Stdin, stdin},
		{`default Agent.stdout`, os.Stdout, stdout},
		{`default Agent.stderr`, os.Stderr, stderr},
	}

	for _, c := range cases {
		if c.expect != c.got {
			t.Errorf("%s? expected '%v', got '%v'", c.name, c.expect, c.got)
		}
	}
}

func TestFirstAction(t *testing.T) {
	nrwc := NopReadWriteCloser{
		Reader: strings.NewReader("{}\n"),
	}
	ag, err := NewAgent(AgentConfig{
		Stdin:  nrwc,
		Stdout: nrwc,
		Stderr: nrwc,
	})
	if err != nil {
		t.Errorf("agent creation failed: %s", err)
		return
	}

	actions := make(chan Action, 1)
	ag.Store.Use(Reduce(func(mx *Ctx) *State {
		select {
		case actions <- mx.Action:
		default:
		}
		return mx.State
	}))

	// there is a small chance that some other package might dispatch an action
	// before we're ready e.g. in init()
	type impossibru struct{ ActionType }
	ag.Store.Dispatch(impossibru{})

	go ag.Run()
	act := <-actions
	switch act.(type) {
	case Started:
	default:
		t.Errorf("Expected first action to be `%T`, but it was %T\n", Started{}, act)
	}
}
