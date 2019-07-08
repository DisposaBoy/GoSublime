package mg

import (
	"bytes"
	"fmt"
	"margo.sh/mgpf"
	"margo.sh/mgutil"
	"sync"
	"time"
)

var (
	evnFrames = []rune{'🄌', '➊', '➋', '➌', '➍', '➎', '➏', '➐', '➑', '➒'}
	oddFrames = []rune{'🄋', '➀', '➁', '➂', '➃', '➄', '➅', '➆', '➇', '➈'}
)

type Task struct {
	Title    string
	Cancel   func()
	CancelID string
	ShowNow  bool
	NoEcho   bool
}

type TaskTicket struct {
	Task
	ID    string
	Start time.Time

	tracker *taskTracker
}

func (ti *TaskTicket) Done() {
	if ti.tracker != nil {
		ti.tracker.done(ti.ID)
	}
}

func (ti *TaskTicket) Cancel() {
	if f := ti.Task.Cancel; f != nil {
		f()
	}
}

func (ti *TaskTicket) Cancellable() bool {
	return ti.Task.Cancel != nil
}

type taskTracker struct {
	ReducerType
	mu       sync.Mutex
	id       uint64
	tickets  []*TaskTicket
	buf      bytes.Buffer
	dispatch Dispatcher
	status   string
	timer    *time.Timer
}

func (tr *taskTracker) RInit(mx *Ctx) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	tr.dispatch = mx.Store.Dispatch
}

func (tr *taskTracker) RUnmount(*Ctx) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	for _, t := range tr.tickets {
		t.Cancel()
	}
}

func (tr *taskTracker) Reduce(mx *Ctx) *State {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	st := mx.State
	switch mx.Action.(type) {
	case RunCmd:
		st = tr.runCmd(st)
	case QueryUserCmds:
		st = tr.userCmds(st)
	}
	if tr.status != "" {
		st = st.AddStatus(tr.status)
	}
	return st
}

func (tr *taskTracker) resetTimer() {
	d := 1 * time.Second
	if tr.timer == nil {
		tr.timer = time.NewTimer(d)
		go tr.ticker()
	} else {
		tr.timer.Reset(d)
	}
}

func (tr *taskTracker) ticker() {
	for range tr.timer.C {
		tr.tick()
	}
}

func (tr *taskTracker) tick() {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	status := tr.render()
	if status != tr.status {
		tr.status = status
		if disp := tr.dispatch; disp != nil {
			disp(Render)
		}
	}
	if len(tr.tickets) != 0 {
		tr.resetTimer()
	}
}

func (tr *taskTracker) userCmds(st *State) *State {
	cl := make([]UserCmd, len(tr.tickets))
	now := time.Now()
	for i, t := range tr.tickets {
		c := UserCmd{Name: ".kill"}
		dur := mgpf.D(now.Sub(t.Start))
		if cid := t.CancelID; cid == "" {
			c.Title = "Task: " + t.Title
			c.Desc = fmt.Sprintf("elapsed: %s", dur)
		} else {
			c.Args = []string{cid}
			c.Title = "Task: Cancel " + t.Title
			c.Desc = fmt.Sprintf("elapsed: %s, cmd: `%s`", dur, mgutil.QuoteCmd(c.Name, c.Args...))
		}
		cl[i] = c
	}
	return st.AddUserCmds(cl...)
}

func (tr *taskTracker) runCmd(st *State) *State {
	return st.AddBuiltinCmds(
		BuiltinCmd{
			Name: ".kill",
			Desc: "List and cancel active tasks",
			Run:  tr.killBuiltin,
		},
	)
}

// Cancel cancels the task tid.
// true is returned if the task exists and was canceled
func (tr *taskTracker) Cancel(tid string) bool {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	return tr.cancel(tid)
}

func (tr *taskTracker) cancel(tid string) bool {
	for _, t := range tr.tickets {
		if t.ID == tid || t.CancelID == tid {
			t.Cancel()
			return t.Cancellable()
		}
	}
	return false
}

func (tr *taskTracker) killBuiltin(cx *CmdCtx) *State {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	defer cx.Output.Close()
	if len(cx.Args) == 0 {
		tr.listAll(cx)
	} else {
		tr.killAll(cx)
	}

	return cx.State
}

func (tr *taskTracker) killAll(cx *CmdCtx) {
	buf := &bytes.Buffer{}
	for _, tid := range cx.Args {
		fmt.Fprintf(buf, "%s: %v\n", tid, tr.cancel(tid))
	}
	cx.Output.Write(buf.Bytes())
}

func (tr *taskTracker) listAll(cx *CmdCtx) {
	buf := &bytes.Buffer{}
	for _, t := range tr.tickets {
		id := t.ID
		if t.CancelID != "" {
			id += "|" + t.CancelID
		}

		dur := time.Since(t.Start)
		if dur < time.Second {
			dur = dur.Round(time.Millisecond)
		} else {
			dur = dur.Round(time.Second)
		}

		fmt.Fprintf(buf, "ID: %s, Dur: %s, Title: %s\n", id, dur, t.Title)
	}
	cx.Output.Write(buf.Bytes())
}

func (tr *taskTracker) drawDigits(n int, f func(int)) {
	if n < 10 {
		f(n)
		return
	}
	m := n / 10
	tr.drawDigits(m, f)
	f(n - m*10)
}

func (tr *taskTracker) render() string {
	if len(tr.tickets) == 0 {
		return ""
	}
	now := time.Now()
	visible := false
	showAnim := false
	title := ""
	for _, t := range tr.tickets {
		dur := now.Sub(t.Start)
		if dur < 1*time.Second {
			continue
		}
		visible = true
		if t.NoEcho || t.Title == "" {
			continue
		}
		if dur < 16*time.Second {
			showAnim = true
		}
		if dur < 8*time.Second {
			title = t.Title
			break
		}
	}
	if !visible {
		return ""
	}
	tr.buf.Reset()
	tr.buf.WriteString("Tasks ")
	frames := oddFrames
	if now.Second()%2 == 0 || !showAnim {
		frames = evnFrames
	}
	tr.drawDigits(len(tr.tickets), func(n int) {
		tr.buf.WriteRune(frames[n])
	})
	if title != "" {
		tr.buf.WriteByte(' ')
		tr.buf.WriteString(title)
	}
	return tr.buf.String()
}

func (tr *taskTracker) done(id string) {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	l := make([]*TaskTicket, 0, len(tr.tickets)-1)
	for _, t := range tr.tickets {
		if t.ID != id {
			l = append(l, t)
		}
	}
	tr.tickets = l
}

func (tr *taskTracker) Begin(o Task) *TaskTicket {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	if cid := o.CancelID; cid != "" {
		for _, t := range tr.tickets {
			if t.CancelID == cid {
				t.Cancel()
			}
		}
	}

	tr.id++
	id := fmt.Sprintf("@%d", tr.id)
	if o.CancelID == "" {
		o.CancelID = id
	}
	t := &TaskTicket{
		Task:    o,
		ID:      id,
		Start:   time.Now(),
		tracker: tr,
	}
	tr.tickets = append(tr.tickets, t)
	tr.resetTimer()
	return t
}
