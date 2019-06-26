package golang

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strconv"
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

func (l impSpecList) mergeWithSrc(fn string, src []byte) (updatedSrc []byte, mergedImports impSpecList, err error) {
	// modifying the AST in areas near comments is a losing battle
	// so we're trying a different strategy:
	// * `import "C"` is ignored as usual
	// * if there are no other imports:
	//   insert `import ("P")\n` below the `package` line
	// * if there is an `import ("X")`:
	//   insert `import ("P";X")`
	// * if there is an `import "X"`:
	//   insert `import ("P";"X"\n)\n`

	eol := func(src []byte, pos int) int {
		if i := bytes.IndexByte(src[pos:], '\n'); i >= 0 {
			return pos + i + 1
		}
		return len(src)
	}

	fset, af, err := parseImportsOnly(fn, src)
	if err != nil {
		return nil, nil, err
	}
	tf := fset.File(af.Pos())
	tailPos := eol(src, tf.Offset(af.End()))

	var target *ast.GenDecl
	skip := map[impSpec]bool{}
	for _, decl := range af.Decls {
		decl, ok := decl.(*ast.GenDecl)
		if !ok || decl.Tok != token.IMPORT {
			continue
		}
		for _, spec := range decl.Specs {
			spec, ok := spec.(*ast.ImportSpec)
			if !ok || spec.Path == nil {
				continue
			}
			imp := impSpec{}
			imp.Path, _ = strconv.Unquote(spec.Path.Value)
			if spec.Name != nil {
				imp.Name = spec.Name.Name
			}
			skip[imp] = true
			if imp.Path != "C" && target == nil {
				target = decl
			}
		}
	}

	out := &bytes.Buffer{}
	merge := func() {
		for _, imp := range l {
			if skip[imp] {
				continue
			}
			skip[imp] = true

			if imp.Name != "" {
				out.WriteString(imp.Name)
			}
			out.WriteString(strconv.Quote(imp.Path))
			out.WriteByte(';')
			mergedImports = append(mergedImports, imp)
		}
	}

	switch {
	case target == nil:
		i := eol(src, tf.Offset(af.Name.End()))
		out.Write(src[:i])
		out.WriteString("\nimport (")
		merge()
		out.WriteString(")\n")
		out.Write(src[i:])
	case target.Lparen > target.TokPos:
		i := tf.Offset(target.Lparen) + 1
		out.Write(src[:i])
		merge()
		out.Write(src[i:])
	default:
		i := tf.Offset(target.TokPos) + len("import")
		j := eol(src, i)
		out.Write(src[:i])
		out.WriteString("(")
		merge()
		out.Write(src[i:j])
		out.WriteString("\n)\n")
		out.Write(src[j:])
	}

	fset, af, err = parseImportsOnly(fn, out.Bytes())
	if err != nil {
		return nil, nil, err
	}

	out.Reset()
	pr := &printer.Config{
		Tabwidth: 4,
		Mode:     printer.TabIndent | printer.UseSpaces,
	}
	if err := pr.Fprint(out, fset, af); err != nil {
		return nil, nil, err
	}
	out.Write(src[tailPos:])
	return out.Bytes(), mergedImports, nil
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

func parseImportsOnly(fn string, src []byte) (*token.FileSet, *ast.File, error) {
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, fn, src, parser.ParseComments|parser.ImportsOnly)
	return fset, af, err
}
