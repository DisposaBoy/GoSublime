package mg

import (
	"io"
	"margo.sh/mgutil"
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
	ag, err := NewAgent(AgentConfig{
		Codec: "invalidcodec",
	})
	if err == nil {
		t.Error("NewAgent() = (nil); want (error)")
	}
	if ag == nil {
		t.Fatal("ag = (nil); want (*Agent)")
	}
	if ag.handle != codecHandles[DefaultCodec] {
		t.Errorf("ag.handle = (%v), want (%v)", ag.handle, codecHandles[DefaultCodec])
	}

	ag, err = NewAgent(AgentConfig{})
	if err != nil {
		t.Fatalf("agent creation failed: %s", err)
	}

	var stdin io.Reader = ag.stdin
	if w, ok := stdin.(*mgutil.IOWrapper); ok {
		stdin = w.Reader
	}
	var stdout io.Writer = ag.stdout
	if w, ok := stdout.(*mgutil.IOWrapper); ok {
		stdout = w.Writer
	}
	var stderr io.Writer = ag.stderr
	if w, ok := stderr.(*mgutil.IOWrapper); ok {
		stderr = w.Writer
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
		t.Run(c.name, func(t *testing.T) {
			if c.expect != c.got {
				t.Errorf("expected '%v', got '%v'", c.expect, c.got)
			}
		})
	}
}

func TestFirstAction(t *testing.T) {
	nrwc := &mgutil.IOWrapper{
		Reader: strings.NewReader("{}\n"),
	}
	ag, err := NewAgent(AgentConfig{
		Stdin:  nrwc,
		Stdout: nrwc,
		Stderr: nrwc,
	})
	if err != nil {
		t.Fatalf("agent creation failed: %s", err)
	}

	actions := make(chan Action, 1)
	ag.Store.Use(NewReducer(func(mx *Ctx) *State {
		select {
		case actions <- mx.Action:
		default:
		}
		return mx.State
	}))
}

type readWriteCloseStub struct {
	mgutil.IOWrapper
	closed    bool
	CloseFunc func() error
}

func (r *readWriteCloseStub) Close() error { return r.CloseFunc() }

func TestAgentShutdown(t *testing.T) {
	nrc := &readWriteCloseStub{}
	nwc := &readWriteCloseStub{}
	nerrc := &readWriteCloseStub{}
	nrc.CloseFunc = func() error {
		nrc.closed = true
		return nil
	}
	nwc.CloseFunc = func() error {
		nwc.closed = true
		return nil
	}
	nerrc.CloseFunc = func() error {
		nerrc.closed = true
		return nil
	}

	ag, err := NewAgent(AgentConfig{
		Stdin:  nrc,
		Stdout: nwc,
		Stderr: nerrc,
		Codec:  "msgpack",
	})
	if err != nil {
		t.Fatalf("agent creation: err = (%#v); want (nil)", err)
	}
	ag.Store = newStore(ag, ag.sub)
	err = ag.Run()
	if err != nil {
		t.Fatalf("ag.Run() = (%#v); want (nil)", err)
	}

	if !nrc.closed {
		t.Error("nrc.Close() want not called")
	}
	if !nwc.closed {
		t.Error("nwc.Close() want not called")
	}
	if !ag.sd.closed {
		t.Error("ag.sd.closed = (true); want (false)")
	}
}
