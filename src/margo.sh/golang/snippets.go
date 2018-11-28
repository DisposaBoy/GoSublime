package golang

import (
	"go/ast"
	"margo.sh/golang/goutil"
	"margo.sh/golang/snippets"
	"margo.sh/mg"
	"sort"
	"strings"
)

var (
	Snippets = SnippetFuncs(append([]snippets.SnippetFunc{ImportPathSnippet}, snippets.DefaultSnippets...)...)
)

// SnippetFunc is an alias of snippets.SnippetFunc
type SnippetFunc = snippets.SnippetFunc

type SnippetFuncsList struct {
	mg.ReducerType
	Funcs []SnippetFunc
}

func SnippetFuncs(l ...SnippetFunc) *SnippetFuncsList {
	return &SnippetFuncsList{Funcs: l}
}

func (sf *SnippetFuncsList) RCond(mx *mg.Ctx) bool {
	return mx.ActionIs(mg.QueryCompletions{}) && mx.LangIs(mg.Go)
}

func (sf *SnippetFuncsList) Reduce(mx *mg.Ctx) *mg.State {
	cx := NewViewCursorCtx(mx)
	var cl []mg.Completion
	for _, f := range sf.Funcs {
		cl = append(cl, f(cx)...)
	}
	for i, _ := range cl {
		sf.fixCompletion(&cl[i])
	}
	return mx.State.AddCompletions(cl...)
}

func (sf *SnippetFuncsList) fixCompletion(c *mg.Completion) {
	c.Src = goutil.DedentCompletion(c.Src)
	if c.Tag == "" {
		c.Tag = mg.SnippetTag
	}
}

// PackageNameSnippet is an alias of snippets.PackageNameSnippet
func PackageNameSnippet(cx *CompletionCtx) []mg.Completion { return snippets.PackageNameSnippet(cx) }

// MainFuncSnippet is an alias of snippets.MainFuncSnippet
func MainFuncSnippet(cx *CompletionCtx) []mg.Completion { return snippets.MainFuncSnippet(cx) }

// InitFuncSnippet is an alias of snippets.InitFuncSnippet
func InitFuncSnippet(cx *CompletionCtx) []mg.Completion { return snippets.InitFuncSnippet(cx) }

// FuncSnippet is an alias of snippets.FuncSnippet
func FuncSnippet(cx *CompletionCtx) []mg.Completion { return snippets.FuncSnippet(cx) }

// MethodSnippet is an alias of snippets.MethodSnippet
func MethodSnippet(cx *CompletionCtx) []mg.Completion { return snippets.MethodSnippet(cx) }

// GenDeclSnippet is an alias of snippets.GenDeclSnippet
func GenDeclSnippet(cx *CompletionCtx) []mg.Completion { return snippets.GenDeclSnippet(cx) }

// MapSnippet is an alias of snippets.MapSnippet
func MapSnippet(cx *CompletionCtx) []mg.Completion { return snippets.MapSnippet(cx) }

// TypeSnippet is an alias of snippets.TypeSnippet
func TypeSnippet(cx *CompletionCtx) []mg.Completion { return snippets.TypeSnippet(cx) }

// AppendSnippet is an alias of snippets.AppendSnippet
func AppendSnippet(cx *CompletionCtx) []mg.Completion { return snippets.AppendSnippet(cx) }

// DocSnippet is an alias of snippets.DocSnippet
func DocSnippet(cx *CompletionCtx) []mg.Completion { return snippets.DocSnippet(cx) }

func ImportPathSnippet(cx *CompletionCtx) []mg.Completion {
	lit, ok := cx.Node.(*ast.BasicLit)
	if !ok || !cx.Scope.Is(ImportPathScope) {
		return nil
	}

	pfx := unquote(lit.Value)
	if i := strings.LastIndexByte(pfx, '/'); i >= 0 {
		pfx = pfx[:i+1]
	} else {
		// if there's no slash, don't do any filtering
		// this allows the fuzzy selection to work in editor
		pfx = ""
	}

	pkl := mctl.plst.View().List
	skip := map[string]bool{}
	srcDir := cx.View.Dir()
	for _, spec := range cx.AstFile.Imports {
		skip[unquote(spec.Path.Value)] = true
	}

	cl := make([]mg.Completion, 0, len(pkl))
	for _, p := range pkl {
		if skip[p.ImportPath] || !p.Importable(srcDir) {
			continue
		}

		src := p.ImportPath
		if pfx != "" {
			src = strings.TrimPrefix(p.ImportPath, pfx)
			if src == p.ImportPath || src == "" {
				continue
			}

			// BUG: in ST
			// given candidate `margo.sh/xxx`, and prefix `margo.sh`
			// if we return xxx, it will replace the whole path
			if !strings.ContainsRune(src, '/') {
				src = p.ImportPath
			}
		}
		cl = append(cl, mg.Completion{
			Query: p.ImportPath,
			Src:   src,
		})
	}
	sort.Slice(cl, func(i, j int) bool { return cl[i].Query < cl[j].Query })
	return cl
}

// DeferSnippet is an alias of snippets.DeferSnippet
func DeferSnippet(cx *CompletionCtx) []mg.Completion { return snippets.DeferSnippet(cx) }
