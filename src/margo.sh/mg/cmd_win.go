// +build windows

package mg

import (
	"os"
	"syscall"
)

var (
	pgSysProcAttr *syscall.SysProcAttr
)

func pgKill(p *os.Process) {
	if p != nil {
		p.Kill()
	}
}
