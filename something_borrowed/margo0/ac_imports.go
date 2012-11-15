package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

type ImportDecl struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type ImportDeclArg struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Add  bool   `json:"add"`
}

type ImportsResult struct {
	Src     string `json:"src"`
	LineRef int    `json:"line_ref"`
}

type ImportsArgs struct {
	Fn        string          `json:"fn"`
	Src       string          `json:"src"`
	Toggle    []ImportDeclArg `json:"toggle"`
	TabWidth  int             `json:"tab_width"`
	TabIndent bool            `json:"tab_indent"`
}

func unquote(s string) string {
	return strings.Trim(s, "\"`")
}

func quote(s string) string {
	return `"` + unquote(s) + `"`
}

func init() {
	act(Action{
		Path: "/imports",
		Doc:  "",
		Func: func(r Request) (data, error) {
			res := ImportsResult{}

			a := ImportsArgs{
				Toggle:    []ImportDeclArg{},
				TabWidth:  8,
				TabIndent: true,
			}

			if err := r.Decode(&a); err != nil {
				return res, err
			}

			fset, af, err := parseAstFile(a.Fn, a.Src, parser.ImportsOnly|parser.ParseComments)
			if err == nil {
				// we neither return, nor attempt the whole source because it likely contains
				// syntax errors after the imports... as a result we need to tell the client
				// which part of the original source we edited so they are able to patch their
				// copy of the source...

				// find the last line number before we edit anything so the client can use it as
				// as reference as where to patch
				res.LineRef = fset.Position(af.Name.End()).Line
				if l := len(af.Decls); l > 0 {
					res.LineRef = fset.Position(af.Decls[l-1].End()).Line
				}

				// trim trailing comments so they don't make the line ref incorrect
				for i, c := range af.Comments {
					cp := fset.Position(c.Pos())
					if cp.Line > res.LineRef {
						af.Comments = af.Comments[:i]
						break
					}
				}

				af = imp(fset, af, a.Toggle)
				res.Src, err = printSrc(fset, af, a.TabIndent, a.TabWidth)
			}
			return res, err
		},
	})
}

func imp(fset *token.FileSet, af *ast.File, toggle []ImportDeclArg) *ast.File {
	add := map[ImportDecl]bool{}
	del := map[ImportDecl]bool{}
	for _, sda := range toggle {
		sd := ImportDecl{
			Path: sda.Path,
			Name: sda.Name,
		}
		if sda.Add {
			add[sd] = true
		} else {
			del[sd] = true
		}
	}

	var firstDecl *ast.GenDecl
	imports := map[ImportDecl]bool{}
	for _, decl := range af.Decls {
		if gdecl, ok := decl.(*ast.GenDecl); ok && len(gdecl.Specs) > 0 {
			hasC := false
			sj := 0
			for _, spec := range gdecl.Specs {
				if ispec, ok := spec.(*ast.ImportSpec); ok {
					sd := ImportDecl{
						Path: unquote(ispec.Path.Value),
					}
					if ispec.Name != nil {
						sd.Name = ispec.Name.String()
					}

					if sd.Path == "C" {
						hasC = true
					} else if del[sd] {
						if sj > 0 {
							if lspec, ok := gdecl.Specs[sj-1].(*ast.ImportSpec); ok {
								lspec.EndPos = ispec.Pos()
							}
						}
						continue
					} else {
						imports[sd] = true
					}
				}

				gdecl.Specs[sj] = spec
				sj += 1
			}
			gdecl.Specs = gdecl.Specs[:sj]

			if !hasC && firstDecl == nil {
				firstDecl = gdecl
			}
		}
	}

	if len(add) > 0 {
		if firstDecl == nil {
			firstDecl = &ast.GenDecl{
				Tok:    token.IMPORT,
				Lparen: 1,
			}
			af.Decls = append(af.Decls, firstDecl)
		} else if firstDecl.Lparen == token.NoPos {
			firstDecl.Lparen = 1
		}

		for sd, _ := range add {
			if !imports[sd] {
				ispec := &ast.ImportSpec{
					Path: &ast.BasicLit{
						Value: quote(sd.Path),
						Kind:  token.STRING,
					},
				}
				if sd.Name != "" {
					ispec.Name = &ast.Ident{
						Name: sd.Name,
					}
				}
				firstDecl.Specs = append(firstDecl.Specs, ispec)
				imports[sd] = true
			}
		}
	}

	dj := 0
	for _, decl := range af.Decls {
		if gdecl, ok := decl.(*ast.GenDecl); ok {
			if len(gdecl.Specs) == 0 {
				continue
			}
		}
		af.Decls[dj] = decl
		dj += 1
	}
	af.Decls = af.Decls[:dj]

	ast.SortImports(fset, af)
	return af
}
