package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

type mImportDecl struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type mImportDeclArg struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Add  bool   `json:"add"`
}

type mImports struct {
	Fn        string
	Src       string
	Toggle    []mImportDeclArg
	TabWidth  int
	TabIndent bool
	Env       map[string]string
	Autoinst  bool
}

func (m *mImports) Call() (interface{}, string) {
	lineRef := 0
	src := ""

	fset, af, err := parseAstFile(m.Fn, m.Src, parser.ImportsOnly|parser.ParseComments)
	if err == nil {
		// we neither return, nor attempt the whole source because it likely contains
		// syntax errors after the imports... as a result we need to tell the client
		// which part of the original source we edited so they are able to patch their
		// copy of the source...

		// find the last line number before we edit anything so the client can use it as
		// as reference as where to patch
		lineRef = fset.Position(af.Name.End()).Line
		if l := len(af.Decls); l > 0 {
			lineRef = fset.Position(af.Decls[l-1].End()).Line
		}

		// trim trailing comments so they don't make the line ref incorrect
		for i, c := range af.Comments {
			cp := fset.Position(c.Pos())
			if cp.Line > lineRef {
				af.Comments = af.Comments[:i]
				break
			}
		}

		af = imp(fset, af, m.Toggle)
		src, err = printSrc(fset, af, m.TabIndent, m.TabWidth)
	}

	if m.Autoinst {
		autoInstall(AutoInstOptions{
			Env:         m.Env,
			ImportPaths: fileImportPaths(af),
		})
	}

	res := M{
		"src":     src,
		"lineRef": lineRef,
	}
	return res, errStr(err)
}

func init() {
	registry.Register("imports", func(_ *Broker) Caller {
		return &mImports{
			Toggle:    []mImportDeclArg{},
			TabWidth:  8,
			TabIndent: true,
		}
	})
}

func unquote(s string) string {
	return strings.Trim(s, "\"`")
}

func quote(s string) string {
	return `"` + unquote(s) + `"`
}

func imp(fset *token.FileSet, af *ast.File, toggle []mImportDeclArg) *ast.File {
	add := map[mImportDecl]bool{}
	del := map[mImportDecl]bool{}
	for _, sda := range toggle {
		sd := mImportDecl{
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
	imports := map[mImportDecl]bool{}
	for _, decl := range af.Decls {
		if gdecl, ok := decl.(*ast.GenDecl); ok && len(gdecl.Specs) > 0 {
			hasC := false
			sj := 0
			for _, spec := range gdecl.Specs {
				if ispec, ok := spec.(*ast.ImportSpec); ok {
					sd := mImportDecl{
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

		addSpecs := make([]ast.Spec, 0, len(firstDecl.Specs)+len(add))
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
				addSpecs = append(addSpecs, ispec)
				imports[sd] = true
			}
		}
		firstDecl.Specs = append(addSpecs, firstDecl.Specs...)
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

	return af
}
