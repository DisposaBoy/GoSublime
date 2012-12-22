package main

import (
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
)

type mLint struct {
	Fn  jString
	Src jString
}

func (m *mLint) Call() (interface{}, string) {
	res := M{}
	reports := make([]M, 0)

	fset, af, err := parseAstFile(m.Fn.String(), m.Src.String(), parser.DeclarationErrors)
	if err == nil {
		reports = lintCheckFlagParse(fset, af, reports)
	} else if el, ok := err.(scanner.ErrorList); ok {
		for _, e := range el {
			reports = append(reports, M{
				"row":     e.Pos.Line - 1,
				"col":     e.Pos.Column - 1,
				"message": e.Msg,
				"kind":    "syntax",
			})
		}
	}

	res["reports"] = reports
	return res, ""
}

func init() {
	registry.Register("lint", func(_ *Broker) Caller {
		return &mLint{}
	})
}

func lintCheckFlagParse(fset *token.FileSet, af *ast.File, res []M) []M {
	reps := []M{}
	foundParse := false
	ast.Inspect(af, func(node ast.Node) bool {
		switch c := node.(type) {
		case *ast.CallExpr:
			if sel, ok := c.Fun.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "flag" {
					if sel.Sel.String() == "Parse" {
						foundParse = true
					} else if !foundParse && c != nil {
						tp := fset.Position(c.Pos())
						if tp.IsValid() {
							reps = append(reps, M{
								"row":     tp.Line - 1,
								"col":     tp.Column - 1,
								"message": "Cannot find corresponding call to flag.Parse()",
								"kind":    "flag",
							})
						}
					}
				}
			}
		}
		return !foundParse
	})

	if !foundParse {
		res = append(res, reps...)
	}

	return res
}
