package golang

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
)

type impSpec struct {
	Name string
	Path string
}

type impSpecList []impSpec

func (l impSpecList) contains(p impSpec) bool {
	for _, q := range l {
		if p == q {
			return true
		}
	}
	return false
}

func unquote(s string) string {
	return strings.Trim(s, "\"`")
}

func quote(s string) string {
	return `"` + unquote(s) + `"`
}

func updateImports(fn string, src []byte, add, rem impSpecList) (_ []byte, updated bool) {
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, fn, src, parser.ImportsOnly|parser.ParseComments)
	if err != nil || af.Name == nil || !af.End().IsValid() {
		return src, false
	}
	tf := fset.File(af.Pos())
	ep := tf.Offset(af.End())
	if i := bytes.IndexByte(src[ep:], '\n'); i >= 0 {
		// make sure to include the ImportComment
		ep += i + 1
	} else {
		ep = tf.Size()
	}
	updateImpSpecs(fset, af, ep, add, rem)
	buf := &bytes.Buffer{}
	pr := &printer.Config{Tabwidth: 4, Mode: printer.TabIndent | printer.UseSpaces}
	if pr.Fprint(buf, fset, af) != nil {
		return src, false
	}
	p, s := buf.Bytes(), src[ep:]
	if len(s) >= 2 && s[0] == '\n' && s[1] == '\n' {
		p = bytes.TrimRight(p, "\n")
	}
	return append(p, s...), true
}

func updateImpSpecs(fset *token.FileSet, af *ast.File, ep int, add, rem impSpecList) {
	var firstImpDecl *ast.GenDecl
	imports := map[impSpec]bool{}
	for _, decl := range af.Decls {
		gdecl, ok := decl.(*ast.GenDecl)
		if !ok || gdecl.Tok != token.IMPORT {
			continue
		}
		hasC := false
		i := 0
		for _, spec := range gdecl.Specs {
			ispec, ok := spec.(*ast.ImportSpec)
			if !ok {
				continue
			}

			sd := impSpec{Path: unquote(ispec.Path.Value)}
			if ispec.Name != nil {
				sd.Name = ispec.Name.String()
			}

			switch {
			case sd.Path == "C":
				hasC = true
			case rem.contains(sd):
				if i > 0 {
					if lspec, ok := gdecl.Specs[i-1].(*ast.ImportSpec); ok {
						lspec.EndPos = ispec.Pos()
					}
				}
				continue
			default:
				imports[sd] = true
			}

			gdecl.Specs[i] = spec
			i += 1
		}
		gdecl.Specs = gdecl.Specs[:i]

		if !hasC && firstImpDecl == nil {
			firstImpDecl = gdecl
		}
	}

	if len(add) > 0 {
		if firstImpDecl == nil {
			tf := fset.File(af.Pos())
			firstImpDecl = &ast.GenDecl{TokPos: tf.Pos(ep), Tok: token.IMPORT, Lparen: 1}
			af.Decls = append(af.Decls, firstImpDecl)
		}

		addSpecs := make([]ast.Spec, 0, len(firstImpDecl.Specs)+len(add))
		for _, sd := range add {
			if imports[sd] {
				continue
			}
			imports[sd] = true
			ispec := &ast.ImportSpec{
				Path: &ast.BasicLit{Value: quote(sd.Path), Kind: token.STRING},
			}
			if sd.Name != "" {
				ispec.Name = &ast.Ident{Name: sd.Name}
			}
			addSpecs = append(addSpecs, ispec)
		}
		firstImpDecl.Specs = append(addSpecs, firstImpDecl.Specs...)
	}
}
