package mgutil

import (
	"sync/atomic"
)

type AtomicBool struct{ n int32 }

func (a *AtomicBool) Set(v bool) {
	if v {
		atomic.StoreInt32(&a.n, 1)
	} else {
		atomic.StoreInt32(&a.n, 0)
	}
}

func (a *AtomicBool) IsSet() bool {
	return atomic.LoadInt32(&a.n) != 0
}
