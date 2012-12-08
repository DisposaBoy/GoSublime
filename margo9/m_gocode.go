package main

import (
	"gosublime.org/gocode"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

const (
	mGocodeAddr = "127.0.0.1:57952"
)

var (
	mGocodeVars = struct {
		lck          sync.Mutex
		lastGopath   string
		lastBuiltins string
	}{}
)

type mGocodeOptions struct {
}

type mGocodeComplete struct {
	Env      map[string]string
	Home     string
	Dir      string
	Builtins bool
	Fn       string
	Src      string
	Pos      int
}

func (m *mGocodeOptions) Call() (interface{}, string) {
	res := M{}
	res["options"] = gocode.GoSublimeGocodeOptions()
	return res, ""
}

func (m *mGocodeComplete) Call() (interface{}, string) {
	e := ""
	res := M{}

	if m.Src == "" {
		// this is here for testing, the client should always send the src
		s, _ := ioutil.ReadFile(m.Fn)
		m.Src = string(s)
	}

	if m.Src == "" {
		return res, "No source"
	}

	pos := 0
	for i, _ := range m.Src {
		pos += 1
		if pos > m.Pos {
			pos = i
			break
		}
	}

	src := []byte(m.Src)
	fn := m.Fn
	if !filepath.IsAbs(fn) {
		fn = filepath.Join(orString(m.Dir, m.Home), orString(fn, "_.go"))
	}

	mGocodeVars.lck.Lock()
	defer mGocodeVars.lck.Unlock()

	builtins := "false"
	if m.Builtins {
		builtins = "true"
	}
	if mGocodeVars.lastBuiltins != builtins {
		gocode.GoSublimeGocodeSet("propose-builtins", builtins)
	}

	gopath := orString(m.Env["GOPATH"], os.Getenv("GOPATH"))
	if gopath != mGocodeVars.lastGopath {
		p := []string{}
		osArch := runtime.GOOS + "_" + runtime.GOARCH
		for _, s := range filepath.SplitList(gopath) {
			p = append(p, filepath.Join(s, "pkg", osArch))
		}
		libpath := strings.Join(p, string(filepath.ListSeparator))
		gocode.GoSublimeGocodeSet("lib-path", libpath)
		mGocodeVars.lastGopath = gopath
	}
	res["completions"] = gocode.GoSublimeGocodeComplete(src, fn, pos)

	return res, e
}

func init() {
	registry.Register("gocode_options", func(b *Broker) Caller {
		return &mGocodeOptions{}
	})

	registry.Register("gocode_complete", func(b *Broker) Caller {
		return &mGocodeComplete{}
	})
}
