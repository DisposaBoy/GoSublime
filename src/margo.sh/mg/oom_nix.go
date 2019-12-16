//+build !windows

package mg

import (
	"syscall"
)

func SetMemoryLimit(logs interface {
	Printf(string, ...interface{})
}, b uint64) {
	rlim := &syscall.Rlimit{Cur: b, Max: b}
	if err := syscall.Getrlimit(syscall.RLIMIT_DATA, rlim); err != nil {
		logs.Printf("SetMemoryLimit: cannot get RLIMIT_DATA: %s\n", err)
		return
	}
	rlim.Cur = b
	mib := b / (1 << 20)
	if err := syscall.Setrlimit(syscall.RLIMIT_DATA, rlim); err != nil {
		logs.Printf("SetMemoryLimit: limit=%dMiB, cannot set RLIMIT_DATA: %s\n", rlim.Cur/(1<<20), err)
		return
	}
	// re-read it so we see what it was actually set to
	syscall.Getrlimit(syscall.RLIMIT_DATA, rlim)
	logs.Printf("SetMemoryLimit: limit=%dMiB, RLIMIT_DATA={Cur: %dMiB, Max:%dMiB}\n", mib, rlim.Cur/(1<<20), rlim.Max/(1<<20))
}
