package main

import (
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
)

type mLintReport struct {
	Fn      string
	Row     int
	Col     int
	Message string
	Kind    string
}

type mLint struct {
	Fn     jString
	Src    jString
	Filter []string

	fset    *token.FileSet
	af      *ast.File
	reports []mLintReport
}

var (
	mLinters = map[string]func(kind string, m *mLint){
		"gs.flag.parse": mLintCheckFlagParse,
	}
)

func (m *mLint) Call() (interface{}, string) {
	filterKind := map[string]bool{}
	for _, kind := range m.Filter {
		filterKind[kind] = true
	}

	var err error
	m.reports = []mLintReport{}
	m.fset, m.af, err = parseAstFile(m.Fn.String(), m.Src.String(), parser.DeclarationErrors)
	if err == nil {
		for kind, f := range mLinters {
			if !filterKind[kind] {
				f(kind, m)
			}
		}
	} else if el, ok := err.(scanner.ErrorList); ok && !filterKind["gs.syntax"] {
		for _, e := range el {
			m.reports = append(m.reports, mLintReport{
				Row:     e.Pos.Line - 1,
				Col:     e.Pos.Column - 1,
				Message: e.Msg,
				Kind:    "gs.syntax",
			})
		}
	}

	res := M{
		"reports": m.reports,
	}
	return res, ""
}

func init() {
	registry.Register("lint", func(_ *Broker) Caller {
		return &mLint{}
	})
}

func mLintCheckFlagParse(kind string, m *mLint) {
	reps := []mLintReport{}
	foundParse := false
	ast.Inspect(m.af, func(node ast.Node) bool {
		switch c := node.(type) {
		case *ast.CallExpr:
			if sel, ok := c.Fun.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "flag" {
					if sel.Sel.String() == "Parse" {
						foundParse = true
					} else if !foundParse && c != nil {
						tp := m.fset.Position(c.Pos())
						if tp.IsValid() {
							reps = append(reps, mLintReport{
								Row:     tp.Line - 1,
								Col:     tp.Column - 1,
								Message: "Cannot find corresponding call to flag.Parse()",
								Kind:    kind,
							})
						}
					}
				}
			}
		}
		return !foundParse
	})

	if !foundParse {
		m.reports = append(m.reports, reps...)
	}
}
