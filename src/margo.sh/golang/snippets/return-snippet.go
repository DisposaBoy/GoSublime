package snippets

import (
	"margo.sh/golang/cursor"
	"margo.sh/mg"
)

func ReturnSnippet(cx *cursor.CurCtx) []mg.Completion {
	if !cx.Scope.Is(cursor.BlockScope) {
		return nil
	}

	cl := []mg.Completion{
		mg.Completion{
			Query: `return`,
			Src:   `return $0`,
		},
	}

	return cl
}
