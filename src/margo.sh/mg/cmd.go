package mg

import (
	"bytes"
	"flag"
	"io"
	"margo.sh/mg/actions"
	"margo.sh/mgutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"
)

var (
	_ OutputStream = (*CmdOut)(nil)
	_ OutputStream = (*IssueOut)(nil)
	_ OutputStream = (OutputStreams)(nil)
	_ OutputStream = (*mgutil.IOWrapper)(nil)
)

type ErrorList []error

func (el ErrorList) First() error {
	for _, e := range el {
		if e != nil {
			return e
		}
	}
	return nil
}

func (el ErrorList) Filter() ErrorList {
	if len(el) == 0 {
		return nil
	}
	res := make(ErrorList, 0, len(el))
	for _, e := range el {
		if e != nil {
			res = append(res, e)
		}
	}
	return res
}

func (el ErrorList) Error() string {
	buf := &bytes.Buffer{}
	for _, e := range el {
		if e == nil {
			continue
		}
		if buf.Len() != 0 {
			buf.WriteByte('\n')
		}
		buf.WriteString(e.Error())
	}
	return buf.String()
}

// OutputStream describes an object that's capable of dispatching command output.
//
// An OutputSream is safe for concurrent use.
//
// The main implementation is CmdOut.
type OutputStream interface {
	io.Writer
	io.Closer
	Flush() error
}

// OutputStreams delegates to a list of OutputStreams.
//
// For each method (Write, Close, Flush):
//
// If none of the underlying methods return an error, a nil error is returned.
//
// Otherwise an ErrorList length == len(OutputStreams) is returned.
// For each entry OutputStreams[i], ErrorList[i] contains the error
// returned for the method called on that OutputStream.
type OutputStreams []OutputStream

// Write calls Write() on all each OutputStream
func (sl OutputStreams) Write(p []byte) (int, error) {
	return len(p), sl.forEach(func(s OutputStream) error {
		n, err := s.Write(p)
		if err == nil && n != len(p) {
			return io.ErrShortWrite
		}
		return err
	})
}

// Close calls Close() on all each OutputStream
func (sl OutputStreams) Close() error {
	return sl.forEach(func(s OutputStream) error { return s.Close() })
}

// Flush calls Flush() on all each OutputStream
func (sl OutputStreams) Flush() error {
	return sl.forEach(func(s OutputStream) error { return s.Flush() })
}

// forEach calls f on each entry in the list
// it takes care of implementing the documented error returned by OutputStreams' methods
func (sl OutputStreams) forEach(f func(OutputStream) error) error {
	var el ErrorList
	for i, s := range sl {
		err := f(s)
		if err == nil {
			continue
		}
		if len(el) == 0 {
			el = make(ErrorList, len(sl))
		}
		el[i] = err
	}
	if len(el) == 0 {
		return nil
	}
	return el
}

type CmdOut struct {
	Fd       string
	Dispatch Dispatcher

	mu     sync.Mutex
	buf    []byte
	closed bool
}

func (w *CmdOut) Write(p []byte) (int, error) {
	return w.write(false, p)
}

func (w *CmdOut) write(writeIfClosed bool, p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed && !writeIfClosed {
		return 0, os.ErrClosed
	}

	w.buf = append(w.buf, p...)
	return len(p), nil
}

// Close closes the writer.
// It returns os.ErrClosed if Close has already been called.
func (w *CmdOut) Close() error {
	defer w.Flush()

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return os.ErrClosed
	}

	w.closed = true
	return nil
}

// Flush implements OutputStream.Flush
//
// If w.Dispatch is set, it's used to dispatch Output{} actions.
// It never returns an error.
func (w *CmdOut) Flush() error {
	if w.Dispatch == nil || w.Fd == "" {
		return nil
	}

	out := w.Output()
	if len(out.Output) != 0 || out.Close {
		w.Dispatch(out)
	}

	return nil
}

// Output returns the data buffered from previous calls to w.Write() and clears
// the buffer.
func (w *CmdOut) Output() CmdOutput {
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

func (out CmdOutput) ClientAction() actions.ClientData {
	return actions.ClientData{Name: "CmdOutput", Data: out}
}

type cmdSupport struct{ ReducerType }

func (cs *cmdSupport) Reduce(mx *Ctx) *State {
	switch act := mx.Action.(type) {
	case RunCmd:
		return runCmd(mx, act)
	}
	return mx.State
}

func runCmd(mx *Ctx, rc RunCmd) *State {
	rc = rc.Interpolate(mx)
	cx := &CmdCtx{
		Ctx:    mx,
		RunCmd: rc,
		Output: &CmdOut{Fd: rc.Fd, Dispatch: mx.Store.Dispatch},
	}
	defer mx.Profile.Push(cx.Name).Pop()
	return cx.Run()
}

type outputStreamRef struct {
	wg   *sync.WaitGroup
	once sync.Once
	OutputStream
}

func newOutputStreamRef(wg *sync.WaitGroup, w OutputStream) OutputStream {
	wg.Add(1)
	return &outputStreamRef{
		wg:           wg,
		OutputStream: w,
	}
}

func (osr *outputStreamRef) Close() error {
	osr.once.Do(osr.wg.Done)
	return nil
}

type runCmdData struct {
	*Ctx
	RunCmd
}

type RunCmdFlagSet struct {
	RunCmd RunCmd
	*flag.FlagSet
}

func (fs RunCmdFlagSet) Parse() error {
	return fs.FlagSet.Parse(fs.RunCmd.Args)
}

type RunDmc = RunCmd
type RunCmd struct {
	ActionType

	Fd       string
	Input    bool
	Name     string
	Args     []string
	CancelID string
	Prompts  []string
}

func (rc RunCmd) Flags() RunCmdFlagSet {
	return RunCmdFlagSet{
		RunCmd:  rc,
		FlagSet: flag.NewFlagSet(rc.Name, flag.ContinueOnError),
	}
}

func (rc RunCmd) StringFlag(name, value string) string {
	fs := rc.Flags()
	v := fs.String(name, value, "")
	fs.Parse()
	return *v
}

func (rc RunCmd) BoolFlag(name string, value bool) bool {
	fs := rc.Flags()
	v := fs.Bool(name, value, "")
	fs.Parse()
	return *v
}

func (rc RunCmd) IntFlag(name string, value int) int {
	fs := rc.Flags()
	v := fs.Int(name, value, "")
	fs.Parse()
	return *v
}

func (rc RunCmd) Interpolate(mx *Ctx) RunCmd {
	data := runCmdData{
		Ctx:    mx,
		RunCmd: rc,
	}
	tpl := template.New("")
	buf := &bytes.Buffer{}
	rc.Name = rc.interp(data, tpl, buf, rc.Name)
	for i, s := range rc.Args {
		rc.Args[i] = rc.interp(data, tpl, buf, s)
	}
	return rc
}

func (rc RunCmd) interp(data runCmdData, tpl *template.Template, buf *bytes.Buffer, s string) string {
	if strings.Contains(s, "{{") && strings.Contains(s, "}}") {
		if tpl, err := tpl.Parse(s); err == nil {
			buf.Reset()
			if err := tpl.Execute(buf, data); err == nil {
				s = buf.String()
			}
		}
	}
	return os.Expand(s, func(k string) string {
		if v, ok := data.Env[k]; ok {
			return v
		}
		return "${" + k + "}"
	})
}

type Proc struct {
	Title string

	cx     *CmdCtx
	mu     sync.RWMutex
	done   chan struct{}
	closed bool
	cmd    *exec.Cmd
	task   *TaskTicket
	cid    string
}

func newProc(cx *CmdCtx) *Proc {
	cmd := exec.Command(cx.Name, cx.Args...)
	if cx.Input {
		s, _ := cx.View.ReadAll()
		cmd.Stdin = bytes.NewReader(s)
	}
	cmd.Dir = cx.View.Wd
	cmd.Env = cx.Env.Environ()
	cmd.Stdout = cx.Output
	cmd.Stderr = cx.Output
	cmd.SysProcAttr = pgSysProcAttr

	name := filepath.Base(cx.Name)
	args := make([]string, len(cx.Args))
	for i, s := range cx.Args {
		if filepath.IsAbs(s) {
			s = filepath.Base(s)
		}
		args[i] = s
	}

	return &Proc{
		Title: "`" + mgutil.QuoteCmd(name, args...) + "`",
		done:  make(chan struct{}),
		cx:    cx,
		cmd:   cmd,
		cid:   cx.CancelID,
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

	p.task = p.cx.Begin(Task{
		CancelID: p.cid,
		Title:    p.Title,
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
			p.cx.Output.Flush()
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
