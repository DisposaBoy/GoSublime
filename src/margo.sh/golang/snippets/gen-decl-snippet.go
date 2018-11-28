package snippets

import (
	"margo.sh/golang/cursor"
	"margo.sh/mg"
)

func GenDeclSnippet(cx *cursor.CurCtx) []mg.Completion {
	switch cx.Scope {
	case cursor.BlockScope:
		return []mg.Completion{
			{
				Query: `var`,
				Title: `X`,
				Src:   `var ${1:name}`,
			},
			{
				Query: `var`,
				Title: `X = Y`,
				Src:   `var ${1:name} = ${2:value}`,
			},
			{
				Query: `const`,
				Title: `X = Y`,
				Src:   `const ${1:name} = ${2:value}`,
			},
		}
	case cursor.FileScope:
		return []mg.Completion{
			{
				Query: `import`,
				Title: `(...)`,
				Src: `
				import (
					"$0"
				)
			`,
			},
			{
				Query: `var`,
				Title: `(...)`,
				Src: `
				var (
					${1:name} = ${2:value}
				)
			`,
			},
			{
				Query: `const`,
				Title: `(...)`,
				Src: `
				const (
					${1:name} = ${2:value}
				)
			`,
			},
		}
	default:
		return nil
	}
}
