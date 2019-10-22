package mgutil

import (
	"runtime"
)

// MinNumCPU calls Min(runtime.NumCPU(), q...).
func MinNumCPU(q ...int) int {
	return Min(runtime.NumCPU(), q...)
}
