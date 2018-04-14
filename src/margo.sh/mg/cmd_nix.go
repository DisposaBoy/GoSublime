// +build !windows

package mg

import (
	"os"
	"syscall"
)

var (
	pgSysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
)

func pgKill(p *os.Process) {
	if p != nil {
		syscall.Kill(-p.Pid, syscall.SIGINT)
	}
}
