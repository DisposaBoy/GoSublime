//+build windows

package mg

func SetMemoryLimit(logs interface {
	Printf(string, ...interface{})
}, b uint64) {
	logs.Printf("SetMemoryLimit: not supported on Windows")
}
