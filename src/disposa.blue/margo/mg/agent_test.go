package mg

import (
	"io"
	"os"
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

	cases := []struct {
		name   string
		expect interface{}
		got    interface{}
	}{
		{`DefaultCodec == json`, true, DefaultCodec == "json"},
		{`codecHandles[DefaultCodec] exists`, true, codecHandles[DefaultCodec] != nil},
		{`codecHandles[""] == codecHandles[DefaultCodec]`, true, codecHandles[""] == codecHandles[DefaultCodec]},
		{`default Agent.stdin`, os.Stdin, ag.stdin},
		{`default Agent.stdout`, os.Stdout, ag.stdout},
		{`default Agent.stderr`, os.Stderr, ag.stderr},
	}

	for _, c := range cases {
		if c.expect != c.got {
			t.Errorf("%s? expected '%v', got '%v'", c.name, c.expect, c.got)
		}
	}
}

func TestStartedAction(t *testing.T) {
	nrwc := nopReadWriteCloser{}
	ag, err := NewAgent(AgentConfig{
		Stdin:  nrwc,
		Stdout: nrwc,
		Stderr: nrwc,
	})
	if err != nil {
		t.Errorf("agent creation failed: %s", err)
		return
	}

	actions := make(chan Action)
	ag.Store.Use(Reduce(func(mx *Ctx) *State {
		select {
		case actions <- mx.Action:
		default:
		}
		return mx.State
	}))
	go ag.Run()
	act := <-actions
	switch act.(type) {
	case Started:
	default:
		t.Errorf("Expected first action to be `Started`, but it was %T\n", act)
	}
}

type nopReadWriteCloser struct{}

func (nopReadWriteCloser) Read(p []byte) (int, error) {
	return 0, io.EOF
}

func (nopReadWriteCloser) Write(p []byte) (int, error) {
	return len(p), nil
}

func (nopReadWriteCloser) Close() error {
	return nil
}
