package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"gosubli.me/something-borrowed/gocode"
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
	Autoinst bool
	Env      map[string]string
	Home     string
	Dir      string
	Builtins bool
	Fn       string
	Src      string
	Pos      int
	calltip  bool
}

type calltipVisitor struct {
	offset int
	fset   *token.FileSet
	x      *ast.CallExpr
}

func (v *calltipVisitor) Visit(node ast.Node) (w ast.Visitor) {
	if node != nil {
		if x, ok := node.(*ast.CallExpr); ok {
			a := v.fset.Position(node.Pos())
			b := v.fset.Position(node.End())

			if (a.IsValid() && v.offset >= a.Offset) && (!b.IsValid() || v.offset <= b.Offset) {
				v.x = x
			}
		}
	}
	return v
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

	if m.calltip {
		res["calltips"] = completeCalltip(src, fn, pos)
	} else {
		l := gocode.GoSublimeGocodeComplete(src, fn, pos)
		res["completions"] = l

		if m.Autoinst && len(l) == 0 {
			autoInstall(AutoInstOptions{
				Src: m.Src,
				Env: m.Env,
			})
		}
	}

	return res, e
}

func completeCalltip(src []byte, fn string, offset int) []gocode.GoSublimeGocodeCandidate {
	fset := token.NewFileSet()
	af, _ := parser.ParseFile(fset, fn, src, 0)

	if af != nil {
		vis := &calltipVisitor{
			offset: offset,
			fset:   fset,
		}
		ast.Walk(vis, af)

		if vis.x != nil {
			var id *ast.Ident

			switch v := vis.x.Fun.(type) {
			case *ast.Ident:
				id = v
			case *ast.SelectorExpr:
				id = v.Sel
			}

			if id != nil && id.End().IsValid() {
				line := offsetLine(fset, af, offset)
				cp := fset.Position(id.End())
				cr := cp.Offset
				cl := gocode.GoSublimeGocodeComplete(src, fn, cr)

				if (cp.Line == line || line == 0) && len(cl) > 0 {
					for i, c := range cl {
						if strings.EqualFold(id.Name, c.Name) {
							return cl[i : i+1]
						}
					}
				}
			}
		}
	}

	return []gocode.GoSublimeGocodeCandidate{}
}

func offsetLine(fset *token.FileSet, af *ast.File, offset int) (line int) {
	defer func() {
		if err := recover(); err != nil {
			line = 0
		}
	}()
	return fset.File(af.Pos()).Position(token.Pos(offset)).Line
}

func init() {
	registry.Register("gocode_options", func(b *Broker) Caller {
		return &mGocodeOptions{}
	})

	registry.Register("gocode_complete", func(b *Broker) Caller {
		return &mGocodeComplete{}
	})

	registry.Register("gocode_calltip", func(b *Broker) Caller {
		return &mGocodeComplete{calltip: true}
	})
}
