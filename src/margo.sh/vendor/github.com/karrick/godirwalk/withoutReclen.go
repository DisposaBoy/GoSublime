// +build dragonfly

package godirwalk

import "syscall"

func direntReclen(_ *syscall.Dirent, nameLength uint64) uint64 {
	return (16 + nameLength + 1 + 7) &^ 7
}
