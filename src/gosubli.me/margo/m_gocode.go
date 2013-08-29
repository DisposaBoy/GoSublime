package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"gosubli.me/something-borrowed/gocode"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var (
	mGocodeVars = struct {
		sync.Mutex

		opts map[string]string
	}{opts: map[string]string{}}
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

	mGocodeVars.Lock()
	defer mGocodeVars.Unlock()

	m.checkOpts()

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

func (m *mGocodeComplete) checkOpts() {
	opts := map[string]string{
		"propose-builtins": "false",
	}

	if m.Builtins {
		opts["propose-builtins"] = "true"
	}

	osArch := runtime.GOOS + "_" + runtime.GOARCH
	goroot, gopaths := envRootList(m.Env)
	pl := []string{}

	add := func(p string) {
		if p != "" {
			pl = append(pl, filepath.Join(p, "pkg", osArch))
		}
	}

	add(goroot)
	for _, p := range gopaths {
		add(p)
	}

	opts["lib-path"] = strings.Join(pl, string(filepath.ListSeparator))

	for k, v := range opts {
		if mGocodeVars.opts[k] != v {
			mGocodeVars.opts[k] = v
			gocode.GoSublimeGocodeSet(k, v)
		}
	}
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
