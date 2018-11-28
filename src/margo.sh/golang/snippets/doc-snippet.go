package snippets

import (
	"go/ast"
	"margo.sh/golang/cursor"
	"margo.sh/mg"
	yotsuba "margo.sh/why_would_you_make_yotsuba_cry"
)

func DocSnippet(cx *cursor.CurCtx) []mg.Completion {
	if cx.Doc == nil {
		return nil
	}

	var ids []*ast.Ident
	var addNames func(n ast.Node)
	addFieldNames := func(fl *ast.FieldList) {
		if fl == nil {
			return
		}
		for _, f := range fl.List {
			ids = append(ids, f.Names...)
			addNames(f.Type)
		}
	}
	addNames = func(n ast.Node) {
		if yotsuba.IsNil(n) {
			return
		}

		switch x := n.(type) {
		case *ast.GenDecl:
			for _, spec := range x.Specs {
				addNames(spec)
			}
		case *ast.SelectorExpr:
			addNames(x.Sel)
			addNames(x.X)
		case *ast.Ident:
			ids = append(ids, x)
		case *ast.File:
			ids = append(ids, x.Name)
		case *ast.FieldList:
			addFieldNames(x)
		case *ast.Field:
			addNames(x.Type)
			ids = append(ids, x.Names...)
		case *ast.TypeSpec:
			ids = append(ids, x.Name)
		case *ast.FuncDecl:
			ids = append(ids, x.Name)
			addFieldNames(x.Recv)
			if t := x.Type; t != nil {
				addFieldNames(t.Params)
				addFieldNames(t.Results)
			}
		case *ast.ValueSpec:
			addNames(x.Type)
			ids = append(ids, x.Names...)
		}
	}
	addNames(cx.Doc.Node)

	pfx := " "
	// we use View.Pos because cx.Pos might have been changed
	if i := cx.View.Pos - 1; 0 <= i && i < len(cx.Src) && (cx.Src[i] == ' ' || cx.Src[i] == '.') {
		pfx = ""
	}
	sfx := " "
	if i := cx.View.Pos; 0 <= i && i < len(cx.Src) && cx.Src[i] == ' ' {
		sfx = ""
	}

	seen := map[string]bool{}
	cl := make([]mg.Completion, 0, len(ids))
	for _, id := range ids {
		if id == nil || id.Name == "_" || seen[id.Name] {
			continue
		}
		seen[id.Name] = true
		cl = append(cl, mg.Completion{
			Query: id.Name,
			Src:   pfx + id.Name + sfx + `$0`,
		})
	}
	return cl
}
