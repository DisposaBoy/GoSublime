package snippets

import (
	"margo.sh/golang/cursor"
	"margo.sh/mg"
)

func TimeSnippet(cx *cursor.CurCtx) []mg.Completion {
	switch {
	case !cx.ImportsMatch(func(p string) bool { return p == "time" }):
		return nil
	default:
		return nil
	}
}
