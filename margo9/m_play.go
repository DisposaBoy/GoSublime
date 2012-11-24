package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type mPlay struct {
	Args      []string          `json:"args"`
	Dir       string            `json:"dir"`
	Src       string            `json:"src"`
	Env       map[string]string `json:"env"`
	Cid       string            `json:"cid"`
	BuildOnly bool              `json:"build_only"`
	b         *Broker
}

// todo: send the client output as it comes
func (m *mPlay) Call() (interface{}, string) {
	env := envSlice(m.Env)

	tmpDir := m.Env["TMP"]
	if tmpDir == "" {
		tmpDir = os.TempDir()
	}
	tmpDir = filepath.Join(tmpDir, "GoSublime", "play")
	// if this fails then the next operation fails as well so no point in checking this
	os.MkdirAll(tmpDir, 0755)

	dir, err := ioutil.TempDir(tmpDir, "play-")
	if err != nil {
		return nil, err.Error()
	}
	defer os.RemoveAll(dir)

	if m.Src != "" {
		err = ioutil.WriteFile(filepath.Join(dir, "a.go"), []byte(m.Src), 0755)
		if err != nil {
			return nil, err.Error()
		}
		m.Dir = dir
	}

	if m.Args == nil {
		m.Args = []string{}
	}

	if m.Dir == "" {
		return nil, "missing directory"
	}

	if m.Cid == "" {
		m.Cid = "play.auto." + numbers.nextString()
	}

	stdErr := bytes.NewBuffer(nil)
	stdOut := bytes.NewBuffer(nil)
	runCmd := func(name string, args ...string) (M, error) {
		start := time.Now()
		stdOut.Reset()
		stdErr.Reset()
		c := exec.Command(name, args...)
		c.Stdout = stdOut
		c.Stderr = stdErr
		c.Dir = m.Dir
		c.Env = env

		watchCmd(m.Cid, c)
		defer unwatchCmd(m.Cid)

		err := c.Run()
		res := M{
			"out": stdOut.String(),
			"err": stdErr.String(),
			"dur": time.Now().Sub(start).String(),
		}

		return res, err
	}

	fn := filepath.Join(dir, "gosublime.a.exe")
	res, err := runCmd("go", "build", "-o", fn)
	if m.BuildOnly || err != nil {
		return res, errStr(err)
	}

	res, err = runCmd(fn, m.Args...)
	return res, errStr(err)
}

func init() {
	registry.Register("play", func(b *Broker) Caller {
		return &mPlay{
			b:   b,
			Env: map[string]string{},
		}
	})
}
