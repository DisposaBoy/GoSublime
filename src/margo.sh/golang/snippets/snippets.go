package snippets

import (
	"margo.sh/golang/cursor"
	"margo.sh/mg"
)

var (
	DefaultSnippets = []SnippetFunc{
		PackageNameSnippet,
		MainFuncSnippet,
		InitFuncSnippet,
		FuncSnippet,
		MethodSnippet,
		GenDeclSnippet,
		MapSnippet,
		TypeSnippet,
		AppendSnippet,
		DocSnippet,
		DeferSnippet,
		MutexSnippet,
		ReturnSnippet,
		HTTPSnippet,
		GoFuncSnippet,
	}
)

type SnippetFunc func(*cursor.CurCtx) []mg.Completion
