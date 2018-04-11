package mg

import (
	"bytes"
	"fmt"
	"io"
	"margo.sh/mgutil"
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
	io.Writer
	io.Closer
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

	n, err := w.buf.Write(p)
	if w.Writer != nil {
		return w.Writer.Write(p)
	}
	return n, err
}

func (w *CmdOutputWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return os.ErrClosed
	}

	w.closed = true
	if w.Closer != nil {
		return w.Closer.Close()
	}
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
		return cs.runCmd(NewBultinCmdCtx(mx, act))
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

type Proc struct {
	bx   *BultinCmdCtx
	mu   sync.RWMutex
	done chan struct{}
	cmd  *exec.Cmd
	task *TaskTicket
}

func (p *Proc) Cancel() {
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

func (p *Proc) start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.task = p.bx.Begin(Task{
		Title:  fmt.Sprintf("RunCmd `%s`", mgutil.QuoteCmd(p.bx.Name, p.bx.Args...)),
		Cancel: p.Cancel,
	})
	go p.dispatchOutputLoop()
	return p.cmd.Start()
}

func (p *Proc) dispatchOutput() {
	p.mu.Lock()
	defer p.mu.Unlock()

	out := p.bx.Output.Output()
	if len(out.Output) != 0 || out.Close {
		p.bx.Store.Dispatch(out)
	}
}

func (p *Proc) dispatchOutputLoop() {
	for {
		select {
		case <-p.done:
			return
		case <-time.After(1 * time.Second):
			p.dispatchOutput()
		}
	}
}

func (p *Proc) Wait() error {
	defer p.dispatchOutput()
	defer func() {
		p.mu.Lock()
		defer p.mu.Unlock()

		close(p.done)
		p.bx.Output.Close()
		p.task.Done()
	}()

	return p.cmd.Wait()
}
