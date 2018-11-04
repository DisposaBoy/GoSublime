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

func updateImports(fn string, src []byte, add, rem impSpecList) (_ []byte, changed bool) {
	parse := func(src []byte) (_ *token.FileSet, _ *ast.File, endOffset int, ok bool) {
		fset := token.NewFileSet()
		af, err := parser.ParseFile(fset, fn, src, parser.ImportsOnly)
		if err != nil || af == nil {
			return nil, nil, -1, false
		}
		end := endOfDeclsOffset(fset, af)
		if end < 0 || end > len(src) {
			return nil, nil, -1, false
		}
		return fset, af, end, true
	}
	print := func(fset *token.FileSet, af *ast.File) (_ []byte, ok bool) {
		buf := &bytes.Buffer{}
		if err := printer.Fprint(buf, fset, af); err != nil {
			return src, false
		}
		return buf.Bytes(), true
	}
	fset, af, end, ok := parse(src)
	if !ok {
		return src, false
	}
	updateImpSpecs(fset, af, add, rem)
	impSrc, ok := print(fset, af)
	if !ok {
		return src, false
	}
	// parsing+printing might introduce whitespace and other garbage
	// so re-parse to find the end of the imports
	_, _, impEnd, ok := parse(impSrc)
	if !ok {
		return src, false
	}

	return append(impSrc[:impEnd], src[end:]...), true
}

func updateImpSpecs(fset *token.FileSet, af *ast.File, add, rem impSpecList) {
	var firstDecl *ast.GenDecl
	imports := map[impSpec]bool{}
	for _, decl := range af.Decls {
		gdecl, ok := decl.(*ast.GenDecl)
		if !ok || len(gdecl.Specs) == 0 {
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

		if !hasC && firstDecl == nil {
			firstDecl = gdecl
		}
	}

	if len(add) > 0 {
		if firstDecl == nil {
			firstDecl = &ast.GenDecl{Tok: token.IMPORT, Lparen: 1}
			af.Decls = append(af.Decls, firstDecl)
		} else if firstDecl.Lparen == token.NoPos {
			firstDecl.Lparen = 1
		}

		addSpecs := make([]ast.Spec, 0, len(firstDecl.Specs)+len(add))
		for _, sd := range add {
			if imports[sd] {
				continue
			}
			ispec := &ast.ImportSpec{
				Path: &ast.BasicLit{Value: quote(sd.Path), Kind: token.STRING},
			}
			if sd.Name != "" {
				ispec.Name = &ast.Ident{Name: sd.Name}
			}
			addSpecs = append(addSpecs, ispec)
			imports[sd] = true
		}
		firstDecl.Specs = append(addSpecs, firstDecl.Specs...)
	}

	i := 0
	for _, decl := range af.Decls {
		if gdecl, ok := decl.(*ast.GenDecl); ok && len(gdecl.Specs) == 0 {
			continue
		}
		af.Decls[i] = decl
		i += 1
	}
	af.Decls = af.Decls[:i]
}

func endOfDeclsOffset(fset *token.FileSet, af *ast.File) int {
	tf := fset.File(af.Pos())
	if len(af.Decls) == 0 {
		return tf.Position(af.End()).Offset
	}

	end := token.NoPos
	for _, d := range af.Decls {
		n := d.End()
		if n > end {
			end = n
		}
	}
	if end.IsValid() {
		return tf.Position(end).Offset
	}
	return -1
}
