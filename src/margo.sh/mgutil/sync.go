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

type AtomicInt int64

func (i *AtomicInt) N() int64 {
	return atomic.LoadInt64((*int64)(i))
}

func (i *AtomicInt) Set(n int64) {
	atomic.StoreInt64((*int64)(i), n)
}

func (i *AtomicInt) Swap(old, new int64) {
	atomic.CompareAndSwapInt64((*int64)(i), old, new)
}

func (i *AtomicInt) Inc() int64 {
	return atomic.AddInt64((*int64)(i), 1)
}

func (i *AtomicInt) Dec() int64 {
	return atomic.AddInt64((*int64)(i), -1)
}

func (i *AtomicInt) Add(n int64) int64 {
	return atomic.AddInt64((*int64)(i), n)
}

func (i *AtomicInt) Sub(n int64) int64 {
	return atomic.AddInt64((*int64)(i), -n)
}
