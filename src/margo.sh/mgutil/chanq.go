package mgutil

import (
	"fmt"
	"os"
	"runtime/debug"
	"sync"
)

// ChanQ is a bounded queue
type ChanQ struct {
	c      chan interface{}
	mu     sync.Mutex
	closed bool
}

// Put puts v into the queue.
// It removes the oldest value if no space is available.
func (q *ChanQ) Put(v interface{}) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return
	}

	for {
		select {
		case q.c <- v:
			return
		case _, open := <-q.c:
			if !open {
				return
			}
		}
	}
}

// C returns a channel on which values are sent
func (q *ChanQ) C() <-chan interface{} {
	return q.c
}

// Close closes the queue and the channel returned by C().
// closing a closed queue has no effect.
func (q *ChanQ) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return
	}

	q.closed = true
	close(q.c)
}

// NewChanQ creates a new ChanQ
// if cap is less than 1, it panics
func NewChanQ(cap int) *ChanQ {
	if cap < 1 {
		panic("ChanQ cap must be greater than, or equal to, one")
	}
	return &ChanQ{c: make(chan interface{}, cap)}
}

// NewChanQLoop creates a new ChanQ and launches a gorotuine to handle objects received on the channel.
// If cap is less than 1, it panics
// If f panics, the panic is recovered and a stack traced is printed to os.Stderr.
func NewChanQLoop(cap int, f func(v interface{})) *ChanQ {
	q := NewChanQ(cap)
	proc := func(v interface{}) {
		defer func() {
			e := recover()
			if e == nil {
				return
			}
			debug.PrintStack()
			fmt.Fprintf(os.Stderr, "PANIC: ChanQ callback panic: %#v\n", e)
		}()
		f(v)
	}
	go func() {
		for v := range q.C() {
			proc(v)
		}
	}()
	return q
}
