package snippets

import (
	"margo.sh/golang/cursor"
	"margo.sh/mg"
)

func FuncSnippet(cx *cursor.CurCtx) []mg.Completion {
	if cx.Scope == cursor.FileScope || cx.Scope.Is(cursor.FuncDeclScope) {
		comp := mg.Completion{
			Query: `func`,
			Title: `name() {...}`,
			Src: `
				func ${1:name}($2)$3 {
					$0
				}
			`,
		}
		if !cx.IsTestFile {
			return []mg.Completion{comp}
		}
		return []mg.Completion{
			{
				Query: `func Test`,
				Title: `Test() {...}`,
				Src: `
					func Test${1:name}(t *testing.T) {
						$0
					}
				`,
			},
			{
				Query: `func Benchmark`,
				Title: `Benchmark() {...}`,
				Src: `
					func Benchmark${1:name}(b *testing.B) {
						$0
					}
				`,
			},
			{
				Query: `func Example`,
				Title: `Example() {...}`,
				Src: `
					func Example${1:name}() {
						$0

						// Output:
					}
				`,
			},
		}
	}

	if cx.Scope.Is(cursor.BlockScope, cursor.VarScope) {
		return []mg.Completion{{
			Query: `func`,
			Title: `func() {...}`,
			Src: `
				func($1)$2 {
					$3
				}$0
			`,
		}}
	}

	return nil
}
