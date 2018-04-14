package mg

import (
	"bytes"
	"fmt"
	"io"
	"margo.sh/mgutil"
	"os"
	"os/exec"
	"sync"
	"time"
)

type CmdOutputWriter struct {
	io.Writer
	io.Closer
	Fd       string
	Dispatch Dispatcher

	mu     sync.Mutex
	buf    []byte
	closed bool
}

func (w *CmdOutputWriter) Copy(updaters ...func(*CmdOutputWriter)) *CmdOutputWriter {
	p := *w
	p.buf = append([]byte{}, w.buf...)
	p.Writer = nil
	p.Closer = nil
	w = &p
	for _, f := range updaters {
		f(w)
	}
	return w
}

func (w *CmdOutputWriter) Write(p []byte) (int, error) {
	return w.write(false, p)
}

func (w *CmdOutputWriter) write(writeIfClosed bool, p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed && !writeIfClosed {
		return 0, os.ErrClosed
	}

	w.buf = append(w.buf, p...)

	if !w.closed {
		if w.Writer != nil {
			return w.Writer.Write(p)
		}
	}

	return len(p), nil
}

func (w *CmdOutputWriter) Close(output ...[]byte) error {
	defer w.dispatch()

	for _, s := range output {
		w.write(true, s)
	}

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

func (w *CmdOutputWriter) dispatch() {
	if w.Dispatch == nil {
		return
	}

	out := w.Output()
	if len(out.Output) != 0 || out.Close {
		w.Dispatch(out)
	}
}

func (w *CmdOutputWriter) Output() CmdOutput {
	w.mu.Lock()
	defer w.mu.Unlock()

	out := CmdOutput{Fd: w.Fd, Output: w.buf, Close: w.closed}
	w.buf = nil
	return out
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

	Fd       string
	Input    bool
	Name     string
	Args     []string
	CancelID string
}

type Proc struct {
	bx     *BultinCmdCtx
	mu     sync.RWMutex
	done   chan struct{}
	closed bool
	cmd    *exec.Cmd
	task   *TaskTicket
	cid    string
}

func newProc(bx *BultinCmdCtx) *Proc {
	cmd := exec.Command(bx.Name, bx.Args...)
	if bx.Input {
		s, _ := bx.View.ReadAll()
		cmd.Stdin = bytes.NewReader(s)
	}
	cmd.Dir = bx.View.Wd
	cmd.Env = bx.Env.Environ()
	cmd.Stdout = bx.Output
	cmd.Stderr = bx.Output
	cmd.SysProcAttr = pgSysProcAttr
	return &Proc{
		done: make(chan struct{}),
		bx:   bx,
		cmd:  cmd,
		cid:  bx.CancelID,
	}
}

func (p *Proc) Cancel() {
	p.mu.RLock()
	defer p.mu.RUnlock()

	select {
	case <-p.done:
	default:
		pgKill(p.cmd.Process)
	}
}

func (p *Proc) start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.task = p.bx.Begin(Task{
		CancelID: p.cid,
		Title:    fmt.Sprintf("Proc`%s`", mgutil.QuoteCmd(p.bx.Name, p.bx.Args...)),
		Cancel:   p.Cancel,
	})
	go p.dispatcher()

	if err := p.cmd.Start(); err != nil {
		p.close()
		return err
	}
	return nil
}

func (p *Proc) dispatcher() {
	defer p.task.Done()

	for {
		select {
		case <-p.done:
			return
		case <-time.After(1 * time.Second):
			p.bx.Output.dispatch()
		}
	}
}

func (p *Proc) close() {
	if p.closed {
		return
	}
	p.closed = true
	close(p.done)
}

func (p *Proc) Wait() error {
	defer func() {
		p.mu.Lock()
		defer p.mu.Unlock()

		p.close()
	}()

	return p.cmd.Wait()
}
