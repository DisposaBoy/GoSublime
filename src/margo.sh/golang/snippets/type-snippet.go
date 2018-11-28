package snippets

import (
	"margo.sh/golang/cursor"
	"margo.sh/mg"
)

func TypeSnippet(cx *cursor.CurCtx) []mg.Completion {
	if cs := cx.Scope; cs != cursor.FileScope && cs != cursor.BlockScope && !cs.Is(cursor.TypeDeclScope) {
		return nil
	}
	return []mg.Completion{
		{
			Query: `type struct`,
			Title: `struct {}`,
			Src: `
				type ${1:T} struct {
					${2:V}
				}
			`,
		},
		{
			Query: `type`,
			Title: `type T`,
			Src:   `type ${1:T} ${2:V}`,
		},
	}
}
