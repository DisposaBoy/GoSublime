// +build nacl linux js solaris aix darwin freebsd netbsd openbsd

package godirwalk

import "syscall"

func direntReclen(de *syscall.Dirent, _ uint64) uint64 {
	return uint64(de.Reclen)
}
