package mg

import (
	"bytes"
	"fmt"
	"margo.sh/mgpf"
	"margo.sh/mgutil"
	"sync"
	"time"
)

const (
	taskAnimInerval = 500 * time.Millisecond
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
}

func (tr *taskTracker) RInit(mx *Ctx) {
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

func (tr *taskTracker) tick() {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	status, resched := tr.render()
	if status != tr.status {
		tr.status = status
		if disp := tr.dispatch; disp != nil {
			disp(Render)
		} else {
			resched++
		}
	}
	if resched > 0 {
		time.AfterFunc(taskAnimInerval, tr.tick)
	}
}

func (tr *taskTracker) userCmds(st *State) *State {
	cl := make([]UserCmd, len(tr.tickets))
	now := time.Now()
	for i, t := range tr.tickets {
		c := UserCmd{
			Title: "Task: Cancel " + t.Title,
			Name:  ".kill",
		}
		for _, s := range []string{t.CancelID, t.ID} {
			if s != "" {
				c.Args = append(c.Args, s)
			}
		}
		c.Desc = fmt.Sprintf("elapsed: %s, cmd: `%s`",
			mgpf.D(now.Sub(t.Start)), mgutil.QuoteCmd(c.Name, c.Args...),
		)
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

func (tr *taskTracker) render() (status string, fresh int) {
	if len(tr.tickets) == 0 {
		return "", 0
	}

	tr.buf.Reset()
	now := time.Now()
	tr.buf.WriteString("Tasks")
	initLen := tr.buf.Len()
	title := ""
	freshFrames := []string{"", " ◔", " ◑", " ◕"}
	staleFrame := " ●"
	for _, t := range tr.tickets {
		dur := now.Sub(t.Start)
		age := int(dur / time.Second)
		if age == 0 && (t.ShowNow || dur >= taskAnimInerval) {
			age = 1
		}
		if age < len(freshFrames) {
			fresh++
			if !t.NoEcho && title == "" && t.Title != "" {
				title = t.Title
			}
			tr.buf.WriteString(freshFrames[age])
		} else {
			tr.buf.WriteString(staleFrame)
		}
	}
	if tr.buf.Len() == initLen && title == "" {
		return "", fresh
	}
	if title != "" {
		tr.buf.WriteByte(' ')
		tr.buf.WriteString(title)
	}
	return tr.buf.String(), fresh
}

func (tr *taskTracker) done(id string) {
	defer tr.tick()

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
	defer tr.tick()

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
	t := &TaskTicket{
		Task:    o,
		ID:      fmt.Sprintf("@%d", tr.id),
		Start:   time.Now(),
		tracker: tr,
	}
	tr.tickets = append(tr.tickets, t)
	return t
}
