package mg

import (
	"go/build"
	"io"
	"margo.sh/mgutil"
)

// NewTestingAgent creates a new agent for testing
//
// The agent config used is equivalent to:
// * Codec: DefaultCodec
// * Stdin: stdin or &mgutil.IOWrapper{} if nil
// * Stdout: stdout or &mgutil.IOWrapper{} if nil
// * Stderr: stderr or &mgutil.IOWrapper{} if nil
//
// * State.Env is set to mgutil.EnvMap{
// *   "GOROOT": build.Default.GOROOT,
// *   "GOPATH": build.Default.GOPATH,
// * }
func NewTestingAgent(stdin io.ReadCloser, stdout io.WriteCloser, stderr io.WriteCloser) *Agent {
	if stdin == nil {
		stdin = &mgutil.IOWrapper{}
	}
	if stdout == nil {
		stdout = &mgutil.IOWrapper{}
	}
	if stderr == nil {
		stderr = &mgutil.IOWrapper{}
	}
	ag, _ := NewAgent(AgentConfig{
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
	})
	ag.Store.state = ag.Store.state.SetEnv(mgutil.EnvMap{
		"GOROOT": build.Default.GOROOT,
		"GOPATH": build.Default.GOPATH,
	})
	return ag
}

// NewTestingStore creates a new Store for testing
// It's equivalent to NewTestingAgent().Store
func NewTestingStore() *Store {
	return NewTestingAgent(nil, nil, nil).Store
}

// NewTestingCtx creates a new Ctx for testing
// It's equivalent to NewTestingStore().NewCtx()
func NewTestingCtx(act Action) *Ctx {
	return NewTestingStore().NewCtx(act)
}
