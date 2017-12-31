package margo_pkg

import (
	"bytes"
	"go/build"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type mPlay struct {
	Args      []string          `json:"args"`
	Dir       string            `json:"dir"`
	Fn        string            `json:"fn"`
	Src       string            `json:"src"`
	Env       map[string]string `json:"env"`
	Cid       string            `json:"cid"`
	BuildOnly bool              `json:"build_only"`
	b         *Broker
}

// todo: send the client output as it comes
func (m *mPlay) Call() (interface{}, string) {
	env := envSlice(m.Env)
	dir, err := ioutil.TempDir(tempDir(m.Env), "play-")
	if err != nil {
		return nil, err.Error()
	}
	defer os.RemoveAll(dir)

	tmpFn := ""
	if m.Src != "" {
		tmpFn = filepath.Join(dir, "a.go")
		err = ioutil.WriteFile(tmpFn, []byte(m.Src), 0644)
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
	} else {
		killCmd(m.Cid)
	}

	res := M{}
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
			"tmpFn": tmpFn,
			"fn":    m.Fn,
			"out":   jData(stdOut.Bytes()),
			"err":   jData(stdErr.Bytes()),
			"dur":   time.Now().Sub(start).String(),
		}

		return res, err
	}

	if !m.BuildOnly {
		pkg, err := build.ImportDir(m.Dir, 0)
		if err != nil {
			return res, err.Error()
		}

		if !pkg.IsCommand() {
			res, err = runCmd("go", "test")
			return res, errStr(err)
		}
	}

	fn := filepath.Join(dir, "gosublime.a.exe")
	res, err = runCmd("go", "build", "-o", fn)
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
