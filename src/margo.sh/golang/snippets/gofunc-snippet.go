package snippets

import (
	"margo.sh/golang/cursor"
	"margo.sh/mg"
)

func GoFuncSnippet(cx *cursor.CurCtx) []mg.Completion {
	if !cx.Scope.Is(cursor.BlockScope) {
		return nil
	}
	return []mg.Completion{
		mg.Completion{
			Query: `go func`,
			Title: `go func{}`,
			Src: `
				go func() {
					${1}
				}()
				$0
			`,
		},
	}
}
