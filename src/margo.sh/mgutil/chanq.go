package mgutil

import (
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
