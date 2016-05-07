package margo_pkg

import (
	"os/exec"
	"sync"
)

var (
	cmdWatchlist = map[string]*exec.Cmd{}
	cmdWatchLck  = sync.Mutex{}
)

type mKill struct {
	Cid string
}

func (m *mKill) Call() (res interface{}, err string) {
	res = M{
		m.Cid: killCmd(m.Cid),
	}
	return
}

func watchCmd(id string, c *exec.Cmd) bool {
	if id == "" {
		return false
	}

	cmdWatchLck.Lock()
	defer cmdWatchLck.Unlock()

	if _, ok := cmdWatchlist[id]; ok {
		return false
	}
	cmdWatchlist[id] = c
	return true
}

func unwatchCmd(id string) bool {
	if id == "" {
		return false
	}

	cmdWatchLck.Lock()
	defer cmdWatchLck.Unlock()

	if _, ok := cmdWatchlist[id]; ok {
		delete(cmdWatchlist, id)
		return true
	}
	return false
}

func killCmd(id string) bool {
	if id == "" {
		return false
	}

	cmdWatchLck.Lock()
	defer cmdWatchLck.Unlock()

	if c, ok := cmdWatchlist[id]; ok {
		// the primary use-case for these functions are remote requests to cancel the proces
		// so we won't remove it from the map
		c.Process.Kill()
		// neither wait nor release are called because the cmd owner should be waiting on it
		return true
	}
	return false
}

func init() {
	byeDefer(func() {
		cmdWatchLck.Lock()
		defer cmdWatchLck.Unlock()
		for _, c := range cmdWatchlist {
			c.Process.Kill()
			c.Process.Release()
		}
	})

	registry.Register("kill", func(b *Broker) Caller {
		return &mKill{}
	})

}
