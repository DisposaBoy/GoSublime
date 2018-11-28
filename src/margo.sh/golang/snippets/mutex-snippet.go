package snippets

import (
	"go/ast"
	"margo.sh/golang/cursor"
	"margo.sh/golang/goutil"
	"margo.sh/mg"
	"strings"
)

func MutexSnippet(cx *cursor.CurCtx) []mg.Completion {
	x, ok := cx.Node.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	sel, _ := cx.Print(x)
	sel = strings.TrimRightFunc(sel, func(r rune) bool { return r != '.' })
	if sel == "" {
		return nil
	}

	snips := func(lock, unlock string) []mg.Completion {
		return []mg.Completion{
			{
				Query: lock + `; defer ` + unlock,
				Src: goutil.DedentCompletion(`
					` + lock + `()
					defer ` + sel + unlock + `()

					$0
				`),
				Tag: mg.SnippetTag,
			},
			{
				Query: lock + `; ...; ` + unlock,
				Src: goutil.DedentCompletion(`
				` + lock + `()
				$0
				` + sel + unlock + `()
			`),
				Tag: mg.SnippetTag,
			},
		}
	}

	// as a temporary hack, until we have typechecking,
	// we'll rely on the gocode reducer to tell us if this is a lock
	cx.Ctx.Defer(func(mx *mg.Ctx) *mg.State {
		cl := []mg.Completion{}
		for _, c := range mx.State.Completions {
			switch c.Query {
			case "Lock":
				cl = append(cl, snips("Lock", "Unlock")...)
			case "RLock":
				cl = append(cl, snips("RLock", "RUnlock")...)
			}
		}
		return mx.AddCompletions(cl...)
	})
	return nil
}
