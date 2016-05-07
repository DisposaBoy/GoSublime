package margo_pkg

import (
	"disposa.blue/something-borrowed/gocode"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"path/filepath"
	"strings"
)

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

func (m *mGocode) Call() (interface{}, string) {
	if m.Src == "" {
		// this is here for testing, the client should always send the src
		s, _ := ioutil.ReadFile(m.Fn)
		m.Src = string(s)
	}

	if m.Src == "" {
		return nil, "No source"
	}

	pos := bytePos(m.Src, m.Pos)
	if m.Pos < 0 {
		return nil, "Invalid offset"
	}

	src := []byte(m.Src)
	fn := m.Fn
	if !filepath.IsAbs(fn) {
		fn = filepath.Join(orString(m.Dir, m.Home), orString(fn, "_.go"))
	}

	res := struct {
		Candidates []gocode.MargoCandidate
	}{}

	if m.calltip {
		res.Candidates = m.calltips(src, fn, pos)
	} else {
		res.Candidates = m.completions(src, fn, pos)
	}

	if m.Autoinst && len(res.Candidates) == 0 {
		autoInstall(AutoInstOptions{
			Src:           m.Src,
			Env:           m.Env,
			InstallSuffix: m.InstallSuffix,
		})
	}

	return res, ""
}

func (g *mGocode) completions(src []byte, fn string, pos int) []gocode.MargoCandidate {
	c := gocode.MargoConfig{}
	c.InstallSuffix = g.InstallSuffix
	c.Builtins = g.Builtins
	c.GOROOT, c.GOPATHS = envRootList(g.Env)
	return gocode.Margo.Complete(c, src, fn, pos)
}

func (m *mGocode) calltips(src []byte, fn string, offset int) []gocode.MargoCandidate {
	id, fset, af := identAtOffset(src, offset)
	if id != nil {
		cp := fset.Position(id.End())
		if cp.IsValid() {
			line := offsetLine(fset, af, offset)
			cr := cp.Offset
			cl := m.completions(src, fn, cr)

			if (cp.Line == line || line == 0) && len(cl) > 0 {
				for i, c := range cl {
					if strings.EqualFold(id.Name, c.Name) {
						return cl[i : i+1]
					}
				}
			}
		}
	}

	return []gocode.MargoCandidate{}
}

func identAtOffset(src []byte, offset int) (id *ast.Ident, fset *token.FileSet, af *ast.File) {
	fset = token.NewFileSet()
	af, _ = parser.ParseFile(fset, "<stdin>", src, 0)

	if af == nil {
		return
	}

	vis := &calltipVisitor{
		offset: offset,
		fset:   fset,
	}
	ast.Walk(vis, af)

	if vis.x != nil && vis.x.Fun != nil {
		switch v := vis.x.Fun.(type) {
		case *ast.Ident:
			id = v
		case *ast.SelectorExpr:
			id = v.Sel
		}
	}
	return
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
	registry.Register("gocode_complete", func(b *Broker) Caller {
		return &mGocode{}
	})

	registry.Register("gocode_calltip", func(b *Broker) Caller {
		return &mGocode{calltip: true}
	})
}
