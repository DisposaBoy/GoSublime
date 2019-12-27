package snippets

import (
	"margo.sh/golang/cursor"
	"margo.sh/mg"
)

func HTTPSnippet(cx *cursor.CurCtx) []mg.Completion {
	switch {
	case !cx.ImportsMatch(func(p string) bool { return p == "net/http" }):
		return nil
	case cx.Scope.Is(cursor.BlockScope):
		return []mg.Completion{
			mg.Completion{
				Query: `http.HandleFunc`,
				Title: `http.HandleFunc("...", func(w, r))`,
				Src: `
					http.HandleFunc("/${1}", func(w http.ResponseWriter, r *http.Request) {
						$0
					})
				`,
			},
		}
	case cx.Scope.Is(cursor.ExprScope):
		return []mg.Completion{
			mg.Completion{
				Query: `http.HandlerFunc`,
				Title: `http.HandlerFunc(func(w, r))`,
				Src: `
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						$0
					})
				`,
			},
			mg.Completion{
				Query: `func http handler`,
				Title: `func(w, r)`,
				Src: `
					func(w http.ResponseWriter, r *http.Request) {
						$0
					}
				`,
			},
		}
	case cx.Scope.Is(cursor.FileScope):
		return []mg.Completion{
			mg.Completion{
				Query: `func http handler`,
				Title: `func(w, r)`,
				Src: `
					func ${1:name}(w http.ResponseWriter, r *http.Request) {
						$0
					}
				`,
			},
		}
	default:
		return nil
	}
}
