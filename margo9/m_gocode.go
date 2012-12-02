package main

import (
	"io/ioutil"
	"net/rpc"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
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

type mGocode struct {
	cmd      string
	Bin      string
	Env      map[string]string
	Home     string
	Dir      string
	Set      map[string]string
	Complete struct {
		Builtins bool
		Fn       string
		Src      string
		Pos      int
	}
}

func (m *mGocode) Call() (interface{}, string) {
	e := ""
	res := M{}
	c, err := rpc.Dial("tcp", mGocodeAddr)
	if err != nil {
		if serveErr := mGocodeServe(m); serveErr != nil {
			return res, "Error starting gocode server: " + serveErr.Error()
		}

		for i := 0; err != nil && i < 5; i += 1 {
			time.Sleep(20 * time.Millisecond)
			c, err = rpc.Dial("tcp", mGocodeAddr)
		}

		if err != nil {
			return res, "Error connecting to gocode server: " + err.Error()
		}
	}
	defer c.Close()

	for k, v := range m.Set {
		if _, e := mGocodeCmdSet(c, k, v); e != "" {
			logger.Print(e)
		}
	}

	switch m.cmd {
	case "set":
		res, e = mGocodeCmdSet(c, "\x00", "\x00")
	case "complete":
		if m.Complete.Src == "" {
			// this is here for testing, the client should always send the src
			s, _ := ioutil.ReadFile(m.Complete.Fn)
			m.Complete.Src = string(s)
		}

		pos := 0
		for i, _ := range m.Complete.Src {
			pos += 1
			if pos > m.Complete.Pos {
				pos = i
				break
			}
		}

		src := []byte(m.Complete.Src)
		fn := m.Complete.Fn
		if !filepath.IsAbs(fn) {
			fn = filepath.Join(orString(m.Dir, m.Home), orString(fn, "_.go"))
		}

		mGocodeVars.lck.Lock()
		defer mGocodeVars.lck.Unlock()

		builtins := "false"
		if m.Complete.Builtins {
			builtins = "true"
		}
		if mGocodeVars.lastBuiltins != builtins {
			if _, e := mGocodeCmdSet(c, "propose-builtins", builtins); e != "" {
				logger.Print(e)
			} else {
				mGocodeVars.lastBuiltins = builtins
			}
		}

		gopath := orString(m.Env["GOPATH"], os.Getenv("GOPATH"))
		if gopath != mGocodeVars.lastGopath {
			p := []string{}
			osArch := runtime.GOOS + "_" + runtime.GOARCH
			for _, s := range filepath.SplitList(gopath) {
				p = append(p, filepath.Join(s, "pkg", osArch))
			}
			libpath := strings.Join(p, string(filepath.ListSeparator))
			if _, e := mGocodeCmdSet(c, "lib-path", libpath); e != "" {
				logger.Print(e)
			} else {
				mGocodeVars.lastGopath = gopath
			}
		}
		res, e = mGocodeCmdComplete(c, fn, src, pos)
	default:
		panic("Unsupported command: gocode: " + m.cmd)
	}

	return res, e
}

func init() {
	registry.Register("gocode_set", func(b *Broker) Caller {
		return &mGocode{cmd: "set"}
	})

	registry.Register("gocode_complete", func(b *Broker) Caller {
		return &mGocode{cmd: "complete"}
	})
}

func mGocodeServe(m *mGocode) error {
	argv := []string{m.Bin, "-s", "-sock", "tcp", "-addr", mGocodeAddr}
	attr := os.ProcAttr{
		Dir:   m.Home,
		Env:   envSlice(m.Env),
		Files: []*os.File{nil, nil, nil},
	}

	p, err := os.StartProcess(m.Bin, argv, &attr)
	if err == nil {
		byeDefer(func() {
			p.Kill()
		})
		go func() {
			_, err := p.Wait()
			if err != nil {
				logger.Println("gocode failed", err)
			}
		}()
	}
	return err
}

func mGocodeCmdSet(c *rpc.Client, k, v string) (res M, e string) {
	args := struct{ Arg0, Arg1 string }{k, v}
	reply := struct{ Arg0 string }{}

	if err := c.Call("RPC.RPC_set", &args, &reply); err != nil {
		e = "RPC error: " + err.Error()
	}
	options := map[string]string{}
	for _, s := range strings.Split(reply.Arg0, "\n") {
		v := strings.SplitN(strings.TrimSpace(s), " ", 2)
		if len(v) == 2 {
			options[v[0]] = v[1]
		}
	}
	res = M{"options": options}
	return
}

func mGocodeCmdComplete(c *rpc.Client, fn string, src []byte, pos int) (res M, e string) {
	args := struct {
		Arg0 []byte
		Arg1 string
		Arg2 int
	}{src, fn, pos}

	reply := struct {
		Arg0 []candidate
		Arg1 int
	}{}

	if err := c.Call("RPC.RPC_auto_complete", &args, &reply); err != nil {
		e = "RPC error: " + err.Error()
	}

	completions := []M{}
	for _, d := range reply.Arg0 {
		completions = append(completions, M{
			"class": d.Class.String(),
			"type":  d.Type,
			"name":  d.Name,
		})
	}
	res = M{"completions": completions}

	return
}
