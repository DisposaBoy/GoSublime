package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"path/filepath"
	"strings"

	"gosubli.me/something-borrowed/gocode"
)

func init() {
	registry.Register("gocode_complete", func(b *Broker) Caller {
		return &mGocode{}
	})

	registry.Register("gocode_calltip", func(b *Broker) Caller {
		return &mGocode{calltip: true}
	})
}

type mGocode struct {
	Autoinst      bool
	InstallSuffix string
	Env           map[string]string
	Home          string
	Dir           string
	Builtins      bool
	Fn            string
	Src           string
	Pos           int

	calltip bool
}

type gocodeResponse struct {
	Candidates []gocode.MargoCandidate
}

func (m *mGocode) Call() (interface{}, string) {
	if m.Src == "" {
		// this is here for testing, the client should always send the src
		b, err := ioutil.ReadFile(m.Fn)
		if err != nil || len(b) == 0 {
			return nil, "No source"
		}
		m.Src = string(b)
	}
	pos := bytePos(m.Src, m.Pos)
	if m.Pos < 0 {
		return nil, "Invalid offset"
	}
	var candidates []gocode.MargoCandidate
	if m.calltip {
		candidates = m.calltips([]byte(m.Src), m.filepath(), pos)
	} else {
		candidates = m.completions([]byte(m.Src), m.filepath(), pos)
	}
	if m.Autoinst && len(candidates) == 0 {
		autoInstall(AutoInstOptions{
			Src:           m.Src,
			Env:           m.Env,
			InstallSuffix: m.InstallSuffix,
		})
	}
	return gocodeResponse{candidates}, ""
}

func (g *mGocode) completions(src []byte, fn string, pos int) []gocode.MargoCandidate {
	root, path := g.goPaths()
	c := gocode.MargoConfig{
		InstallSuffix: g.InstallSuffix,
		Builtins:      g.Builtins,
		GOROOT:        root,
		GOPATH:        path,
	}
	return gocode.Margo.Complete(src, fn, pos, c)
}

func (g *mGocode) calltips(src []byte, filename string, offset int) []gocode.MargoCandidate {
	fset := token.NewFileSet()
	af, _ := parser.ParseFile(fset, "<stdin>", src, 0)
	if af == nil {
		return emptyCandidate()
	}
	id := calltipsWalk(af, fset, offset)
	if id == nil {
		return emptyCandidate()
	}
	end := fset.Position(id.End())
	if !end.IsValid() {
		return emptyCandidate()
	}
	line := offsetLine(fset, af, offset)
	if end.Line != line && line != 0 {
		return emptyCandidate()
	}
	cl := g.completions(src, filename, end.Offset)
	if len(cl) == 0 {
		return emptyCandidate()
	}
	for i, c := range cl {
		if strings.EqualFold(id.Name, c.Name) {
			return cl[i : i+1]
		}
	}
	return emptyCandidate()
}

func emptyCandidate() []gocode.MargoCandidate {
	return []gocode.MargoCandidate{}
}

func (g *mGocode) filepath() string {
	if filepath.IsAbs(g.Fn) {
		return g.Fn
	}
	return filepath.Join(orString(g.Dir, g.Home), orString(g.Fn, "_.go"))
}

func (g *mGocode) goPaths() (goroot, gopath string) {
	if g.Env != nil {
		goroot = g.Env["GOROOT"]
		gopath = g.Env["GOPATH"]
	}
	return
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
			if a.IsValid() && v.offset >= a.Offset {
				b := v.fset.Position(node.End())
				if !b.IsValid() || v.offset <= b.Offset {
					v.x = x
				}
			}
		}
	}
	return v
}

func calltipsWalk(af *ast.File, fset *token.FileSet, offset int) *ast.Ident {
	vis := &calltipVisitor{
		offset: offset,
		fset:   fset,
	}
	ast.Walk(vis, af)
	if vis.x == nil || vis.x.Fun == nil {
		return nil
	}
	switch v := vis.x.Fun.(type) {
	case *ast.Ident:
		return v
	case *ast.SelectorExpr:
		return v.Sel
	}
	return nil
}

func offsetLine(fset *token.FileSet, af *ast.File, offset int) int {
	if fset == nil || af == nil || !af.Pos().IsValid() {
		return 0
	}
	f := fset.File(af.Pos())
	if f == nil {
		return 0
	}
	// prevent f.Position from panicking
	if offset < f.Base() || offset > f.Base()+f.Size() {
		return 0
	}
	return f.Position(token.Pos(offset)).Line
}
