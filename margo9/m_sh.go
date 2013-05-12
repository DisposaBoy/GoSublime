package main

import (
	"bytes"
	"os/exec"
	"strings"
	"time"
)

type mShCmd struct {
	Name  string
	Args  []string
	Input string
	And   *mShCmd
	Or    *mShCmd
}

type mSh struct {
	Env map[string]string
	Cmd mShCmd
	Cid string
	Cwd string
}

// todo: send the client output as it comes
//       handle And, Or
func (m *mSh) Call() (interface{}, string) {
	env := envSlice(m.Env)

	if m.Cid == "" {
		m.Cid = "sh.auto." + numbers.nextString()
	} else {
		killCmd(m.Cid)
	}

	start := time.Now()
	stdErr := bytes.NewBuffer(nil)
	stdOut := bytes.NewBuffer(nil)
	c := exec.Command(m.Cmd.Name, m.Cmd.Args...)
	c.Stdout = stdOut
	c.Stderr = stdErr
	if m.Cmd.Input != "" {
		c.Stdin = strings.NewReader(m.Cmd.Input)
	}
	c.Dir = m.Cwd
	c.Env = env

	watchCmd(m.Cid, c)
	err := c.Run()
	unwatchCmd(m.Cid)

	res := M{
		"out": jData(stdOut.Bytes()),
		"err": jData(stdErr.Bytes()),
		"dur": time.Now().Sub(start).String(),
	}
	return res, errStr(err)
}

func init() {
	registry.Register("sh", func(b *Broker) Caller {
		return &mSh{}
	})
}
