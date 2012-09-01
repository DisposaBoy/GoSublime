package main

import (
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
)

type AcLintArgs struct {
	Fn  string `json:"fn"`
	Src string `json:"src"`
}

type AcLintReport struct {
	Row int    `json:"row"`
	Col int    `json:"col"`
	Msg string `json:"msg"`
}

func init() {
	act(Action{
		Path: "/lint",
		Doc:  ``,
		Func: func(r Request) (data, error) {
			a := AcLintArgs{}
			res := make([]AcLintReport, 0)

			if err := r.Decode(&a); err != nil {
				return res, err
			}

			fset, af, err := parseAstFile(a.Fn, a.Src, parser.DeclarationErrors)
			if err == nil {
				res = lintCheckFlagParse(fset, af, res)
			} else if el, ok := err.(scanner.ErrorList); ok {
				for _, e := range el {
					res = append(res, AcLintReport{
						Row: e.Pos.Line - 1,
						Col: e.Pos.Column - 1,
						Msg: e.Msg,
					})
				}
			}

			return res, nil
		},
	})
}

func lintCheckFlagParse(fset *token.FileSet, af *ast.File, res []AcLintReport) []AcLintReport {
	reps := []AcLintReport{}
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
							reps = append(reps, AcLintReport{
								Row: tp.Line - 1,
								Col: tp.Column - 1,
								Msg: "Cannot find corresponding call to flag.Parse()",
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
