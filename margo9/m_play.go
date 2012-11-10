package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

type mPlay struct {
	Dir string `json:"dir"`
	Src string
	Env map[string]string `json:"env"`
	b   *Broker
	Cid string
}

// todo: send the client output as it comes
func (m *mPlay) Call() (interface{}, string) {
	env := []string{}
	for k, v := range m.Env {
		env = append(env, k+"="+v)
	}

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

	if m.Dir == "" {
		return nil, "missing directory"
	}

	if m.Cid == "" {
		m.Cid = "play.auto." + numbers.nextString()
	}

	stdErr := bytes.NewBuffer(nil)
	stdOut := bytes.NewBuffer(nil)
	runCmd := func(cid, name string, args ...string) error {

		stdOut.Reset()
		stdErr.Reset()
		c := exec.Command(name, args...)
		c.Stdout = stdOut
		c.Stderr = stdErr
		c.Dir = m.Dir
		c.Env = env

		watchCmd(cid, c)
		defer unwatchCmd(cid)

		return c.Run()
	}

	fn := filepath.Join(dir, "gosublime.a.exe")
	err = runCmd(m.Cid, "go", "build", "-o", fn)

	if err != nil {
		res := M{
			"out": stdOut.String(),
			"err": stdErr.String(),
		}
		return res, err.Error()
	}

	err = runCmd(m.Cid, fn)
	res := M{
		"out": stdOut.String(),
		"err": stdErr.String(),
	}
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
