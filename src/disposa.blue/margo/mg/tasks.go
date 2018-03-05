package mg

import (
	"bytes"
	"fmt"
	"sync"
	"time"
)

type taskTick struct{ ActionType }

type Task struct {
	Title    string
	Cancel   func()
	CancelID string
}

type TaskTicket struct {
	ID       string
	Title    string
	Start    time.Time
	CancelID string

	tracker *taskTracker
	cancel  func()
}

func (ti *TaskTicket) Done() {
	if ti.tracker != nil {
		ti.tracker.done(ti.ID)
	}
}

func (ti *TaskTicket) Cancel() {
	if ti.cancel != nil {
		ti.cancel()
	}
}

func (ti *TaskTicket) Cancellable() bool {
	return ti.cancel != nil
}

type taskTracker struct {
	mu       sync.Mutex
	id       uint64
	tickets  []*TaskTicket
	timer    *time.Timer
	dispatch Dispatcher
	buf      bytes.Buffer
}

func newTaskTracker(dispatch Dispatcher) *taskTracker {
	return &taskTracker{
		timer:    time.NewTimer(1 * time.Second),
		dispatch: dispatch,
	}
}

func (tr *taskTracker) Reduce(mx *Ctx) *State {
	tr.mu.Lock()
	defer tr.mu.Unlock()

	switch mx.Action.(type) {
	case Started:
		tr.start()
	case taskTick:
		if len(tr.tickets) != 0 {
			tr.resetTimer()
		}
	}
	if s := tr.status(); s != "" {
		return mx.AddStatus(s)
	}
	return mx.State
}

func (tr *taskTracker) status() string {
	tr.buf.Reset()
	now := time.Now()
	tr.buf.WriteString("Tasks")
	initLen := tr.buf.Len()
	title := ""
	for _, t := range tr.tickets {
		age := now.Sub(t.Start) / time.Second
		switch age {
		case 0:
		case 1:
			tr.buf.WriteString(" ◔")
		case 2:
			tr.buf.WriteString(" ◑")
		case 3:
			tr.buf.WriteString(" ◕")
		default:
			tr.buf.WriteString(" ●")
		}
		if title == "" && t.Title != "" && age >= 1 && age <= 3 {
			title = t.Title
		}
	}
	if tr.buf.Len() == initLen {
		return ""
	}
	if title != "" {
		tr.buf.WriteByte(' ')
		tr.buf.WriteString(title)
	}
	return tr.buf.String()
}

func (tr *taskTracker) titles() (stale []string, fresh []string) {
	now := time.Now()
	for _, t := range tr.tickets {
		dur := now.Sub(t.Start)
		switch {
		case dur >= 5*time.Second:
			stale = append(stale, t.Title)
		case dur >= 1*time.Second:
			fresh = append(fresh, t.Title)
		}
	}
	for _, t := range tr.tickets {
		dur := now.Sub(t.Start)
		switch {
		case dur >= 5*time.Second:
			stale = append(stale, t.Title)
		case dur >= 1*time.Second:
			fresh = append(fresh, t.Title)
		}
	}
	return stale, fresh
}

func (tr *taskTracker) start() {
	go func() {
		for range tr.timer.C {
			tr.dispatch(taskTick{})
		}
	}()
}

func (tr *taskTracker) resetTimer() {
	defer tr.timer.Reset(1 * time.Second)
}

func (tr *taskTracker) done(id string) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	defer tr.resetTimer()

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
	defer tr.resetTimer()

	if cid := o.CancelID; cid != "" {
		for _, t := range tr.tickets {
			if t.CancelID == cid {
				t.Cancel()
			}
		}
	}

	tr.id++
	t := &TaskTicket{
		ID:       fmt.Sprintf("@%d", tr.id),
		CancelID: o.CancelID,
		Title:    o.Title,
		Start:    time.Now(),
		cancel:   o.Cancel,
		tracker:  tr,
	}
	tr.tickets = append(tr.tickets, t)
	return t
}
