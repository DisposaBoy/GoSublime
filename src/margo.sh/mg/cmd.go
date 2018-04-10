package mg

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

var (
	defaultSysProcAttr *syscall.SysProcAttr
)

type CmdOutputWriter struct {
	Fd string

	mu     sync.Mutex
	buf    bytes.Buffer
	closed bool
}

func (w *CmdOutputWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, os.ErrClosed
	}

	return w.buf.Write(p)
}

func (w *CmdOutputWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return os.ErrClosed
	}
	w.closed = true
	return nil
}

func (w *CmdOutputWriter) Output() CmdOutput {
	w.mu.Lock()
	defer w.mu.Unlock()

	s := w.buf.Bytes()
	w.buf.Reset()
	return CmdOutput{Fd: w.Fd, Output: s, Close: w.closed}
}

type CmdOutput struct {
	ActionType

	Fd     string
	Output []byte
	Close  bool
}

type cmdSupport struct{}

func (cs *cmdSupport) Reduce(mx *Ctx) *State {
	switch act := mx.Action.(type) {
	case RunCmd:
		return cs.runCmd(&BultinCmdCtx{Ctx: mx, RunCmd: act})
	case CmdOutput:
		return cs.cmdOutput(mx, act)
	}
	return mx.State
}

func (cs *cmdSupport) runCmd(bx *BultinCmdCtx) *State {
	if cmd, ok := bx.BuiltinCmds.Lookup(bx.Name); ok {
		return cmd.Run(bx)
	}
	return Builtins.ExecCmd(bx)
}

func (cs *cmdSupport) cmdOutput(mx *Ctx, out CmdOutput) *State {
	return mx.State.addClientActions(clientActionType{
		Name: "output",
		Data: out,
	})
}

type RunCmd struct {
	ActionType

	Fd    string
	Input bool
	Name  string
	Args  []string
}

type proc struct {
	rc   RunCmd
	out  *CmdOutputWriter
	mu   sync.RWMutex
	done chan struct{}
	mx   *Ctx
	cmd  *exec.Cmd
	task *TaskTicket
}

func starCmd(mx *Ctx, rc RunCmd) (*proc, error) {
	cmd := exec.Command(rc.Name, rc.Args...)

	if rc.Input {
		r, err := mx.View.Open()
		if err != nil {
			return nil, err
		}
		cmd.Stdin = r
	}

	out := &CmdOutputWriter{Fd: rc.Fd}
	cmd.Dir = mx.View.Dir()
	cmd.Env = mx.Env.Environ()
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.SysProcAttr = defaultSysProcAttr

	p := &proc{
		rc:   rc,
		done: make(chan struct{}),
		mx:   mx,
		cmd:  cmd,
		out:  out,
	}
	return p, p.start()
}

func (p *proc) Cancel() {
	p.mu.RLock()
	defer p.mu.RUnlock()

	select {
	case <-p.done:
	default:
		if p := p.cmd.Process; p != nil {
			p.Signal(os.Interrupt)
		}
	}
}

func (p *proc) start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.task = p.mx.Begin(Task{
		Title:  fmt.Sprintf("RunCmd{Name: %s, Args: %s}", p.rc.Name, p.rc.Args),
		Cancel: p.Cancel,
	})
	go p.dispatchOutputLoop()
	return p.cmd.Start()
}

func (p *proc) dispatchOutput() {
	p.mu.Lock()
	defer p.mu.Unlock()

	out := p.out.Output()
	if len(out.Output) != 0 || out.Close {
		p.mx.Store.Dispatch(out)
	}
}

func (p *proc) dispatchOutputLoop() {
	for {
		select {
		case <-p.done:
			return
		case <-time.After(1 * time.Second):
			p.dispatchOutput()
		}
	}
}

func (p *proc) Wait() error {
	defer p.dispatchOutput()
	defer func() {
		p.mu.Lock()
		defer p.mu.Unlock()

		close(p.done)
		p.out.Close()
		p.task.Done()
	}()

	return p.cmd.Wait()
}
