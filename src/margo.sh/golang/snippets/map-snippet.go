package snippets

import (
	"margo.sh/golang/cursor"
	"margo.sh/mg"
)

func MapSnippet(cx *cursor.CurCtx) []mg.Completion {
	if !cx.Scope.Is(cursor.ExprScope) {
		return nil
	}
	return []mg.Completion{
		{
			Query: `map`,
			Title: `map[T]T`,
			Src:   `map[${1:T}]${2:T}`,
		},
		{
			Query: `map`,
			Title: `map[T]T{...}`,
			Src:   `map[${1:T}]${2:T}{$0}`,
		},
	}
}
