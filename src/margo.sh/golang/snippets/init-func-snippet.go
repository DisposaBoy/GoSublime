package snippets

import (
	"go/ast"
	"margo.sh/golang/cursor"
	"margo.sh/mg"
)

func InitFuncSnippet(cx *cursor.CurCtx) []mg.Completion {
	if cx.Scope != cursor.FileScope {
		return nil
	}

	for _, x := range cx.AstFile.Decls {
		x, ok := x.(*ast.FuncDecl)
		if ok && x.Name != nil && x.Name.String() == "init" {
			return nil
		}
	}

	return []mg.Completion{{
		Query: `func init`,
		Title: `init() {...}`,
		Src: `
			func init() {
				$0
			}
		`,
	}}
}
