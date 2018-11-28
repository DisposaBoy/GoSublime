package snippets

import (
	"margo.sh/golang/cursor"
	"margo.sh/mg"
)

func DeferSnippet(cx *cursor.CurCtx) []mg.Completion {
	if !cx.Scope.Is(cursor.BlockScope) {
		return nil
	}
	return []mg.Completion{
		mg.Completion{
			Query: `defer func`,
			Title: `defer func{}`,
			Src: `
				defer func() {
					${1}
				}()
				$0
			`,
		},
		mg.Completion{
			Query: `defer`,
			Title: `defer f()`,
			Src: `
				defer ${1:f}()
				$0
			`,
		},
	}
}
