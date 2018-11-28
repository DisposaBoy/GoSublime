package snippets

import (
	"go/ast"
	"margo.sh/golang/cursor"
	"margo.sh/mg"
)

func MainFuncSnippet(cx *cursor.CurCtx) []mg.Completion {
	if cx.Scope != cursor.FileScope || cx.PkgName != "main" {
		return nil
	}

	for _, x := range cx.AstFile.Decls {
		x, ok := x.(*ast.FuncDecl)
		if ok && x.Name != nil && x.Name.String() == "main" {
			return nil
		}
	}

	return []mg.Completion{{
		Query: `func main`,
		Title: `main() {...}`,
		Src: `
			func main() {
				$0
			}
		`,
	}}
}
