// +build posix

package mg

import (
	"syscall"
)

func init() {
	defaultSysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
}
