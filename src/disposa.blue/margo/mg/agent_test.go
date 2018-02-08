package mg

import (
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
