package main

import (
	"go/ast"
	"go/parser"
)

type AcFmtArgs struct {
	Fn        string `json:"fn"`
	Src       string `json:"src"`
	TabIndent bool   `json:"tab_indent"`
	TabWidth  int    `json:"tab_width"`
}

func init() {
	act(Action{
		Path: "/fmt",
		Doc: `
formats the source like gofmt does
@data: {"fn": "...", "src": "..."}
@resp: "formatted source"
`,
		Func: func(r Request) (data, error) {
			a := AcFmtArgs{
				TabIndent: true,
				TabWidth:  8,
			}

			res := ""
			if err := r.Decode(&a); err != nil {
				return res, err
			}

			fset, af, err := parseAstFile(a.Fn, a.Src, parser.ParseComments)
			if err == nil {
				ast.SortImports(fset, af)
				res, err = printSrc(fset, af, a.TabIndent, a.TabWidth)
			}
			return res, err
		},
	})
}
