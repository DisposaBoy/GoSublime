package snippets

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
	"margo.sh/golang/cursor"
	"margo.sh/mg"
)

func AppendSnippet(cx *cursor.CurCtx) []mg.Completion {
	if !cx.Scope.Is(cursor.ExprScope) {
		return nil
	}

	cl := func(sel string) []mg.Completion {
		if sel == "" {
			sel = "s"
		}
		return []mg.Completion{
			mg.Completion{
				Query: `append`,
				Title: `append(` + sel + `, ...)`,
				Src:   `append(${1:` + sel + `}, ${2})$0`,
			},
			mg.Completion{
				Query: `append:len`,
				Title: `append(` + sel + `[:len:len], ...)`,
				Src:   `append(${1:` + sel + `}[:len(${1:` + sel + `}):len(${1:` + sel + `})], ${2})$0`,
			},
		}
	}

	if !cx.Scope.Is(cursor.AssignmentScope) {
		return cl("")
	}

	var asn *ast.AssignStmt
	if !cx.Set(&asn) || len(asn.Lhs) != 1 || len(asn.Rhs) > 1 {
		return cl("")
	}

	sel := ""
	switch x := asn.Lhs[0].(type) {
	case *ast.Ident:
		sel = x.Name
	case *ast.SelectorExpr:
		buf := &bytes.Buffer{}
		printer.Fprint(buf, token.NewFileSet(), x)
		sel = buf.String()
	}
	return cl(sel)
}
