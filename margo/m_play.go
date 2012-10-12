package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

type mPlay struct {
	Dir string            `json:"dir"`
	Env map[string]string `json:"env"`
}

// todo: send the client output as it comes
func (m *mPlay) Call() (interface{}, string) {
	env := []string{}
	for k, v := range m.Env {
		env = append(env, k+"="+v)
	}

	dir, err := ioutil.TempDir(m.Env["TMP"], filepath.Join("GoSublime", "play-"))
	if err != nil {
		return nil, err.Error()
	}
	defer os.RemoveAll(dir)

	stdErr := bytes.NewBuffer(nil)
	stdOut := bytes.NewBuffer(nil)
	runCmd := func(name string, args ...string) error {
		stdOut.Reset()
		stdErr.Reset()
		c := exec.Command(name, args...)
		c.Stdout = stdOut
		c.Stderr = stdErr
		c.Dir = m.Dir
		c.Env = env
		return c.Run()
	}

	fn := filepath.Join(dir, "a.exe")
	err = runCmd("go", "build", "-o", fn)

	if err != nil {
		res := M{
			"out": stdOut.String(),
			"err": stdErr.String(),
		}
		return res, err.Error()
	}

	err = runCmd(fn)
	res := M{
		"out": stdOut.String(),
		"err": stdErr.String(),
	}
	return res, errStr(err)
}

func init() {
	registry.Register("play", func() Caller {
		return &mPlay{
			Env: map[string]string{},
		}
	})
}
