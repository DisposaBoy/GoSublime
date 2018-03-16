package golang

import (
	"disposa.blue/margo/mg"
	"go/ast"
	"unicode"
	"unicode/utf8"
)

var (
	Snippets = SnippetFuncs{
		PackageNameSnippet,
		MainFuncSnippet,
		InitFuncSnippet,
		FuncSnippet,
		MethodSnippet,
		GenDeclSnippet,
		MapSnippet,
		TypeSnippet,
	}
)

type SnippetFuncs []func(*CompletionCtx) []mg.Completion

func (sf SnippetFuncs) Reduce(mx *mg.Ctx) *mg.State {
	if !mx.LangIs("go") || !mx.ActionIs(mg.QueryCompletions{}) {
		return mx.State
	}

	src, _ := mx.View.ReadAll()
	pos := mx.View.Pos
	for {
		r, n := utf8.DecodeLastRune(src[:pos])
		if !IsLetter(r) {
			break
		}
		pos -= n
	}
	cx := NewCompletionCtx(mx, src, pos)
	if cx.Scope.Any(StringScope, ImportPathScope, CommentScope) {
		return mx.State
	}

	var cl []mg.Completion
	for _, f := range sf {
		cl = append(cl, f(cx)...)
	}
	for i, _ := range cl {
		sf.fixCompletion(&cl[i])
	}
	return mx.State.AddCompletions(cl...)
}

func (sf SnippetFuncs) fixCompletion(c *mg.Completion) {
	c.Src = DedentCompletion(c.Src)
	if c.Tag == "" {
		c.Tag = mg.SnippetTag
	}
}

func PackageNameSnippet(cx *CompletionCtx) []mg.Completion {
	if cx.PkgName != "" || !cx.Scope.Is(PackageScope) {
		return nil
	}

	name := "main"
	bx := BuildContext(cx.Ctx)
	pkg, _ := bx.ImportDir(cx.View.Dir(), 0)
	if pkg != nil && pkg.Name != "" {
		name = pkg.Name
	}

	return []mg.Completion{{
		Query: `package ` + name,
		Src: `
			package ` + name + `

			$0
		`,
	}}
}

func MainFuncSnippet(cx *CompletionCtx) []mg.Completion {
	if !cx.Scope.Is(FileScope) || cx.PkgName != "main" {
		return nil
	}

	for _, x := range cx.AstFile.Decls {
		x, ok := x.(*ast.FuncDecl)
		if ok && x.Name != nil && x.Name.String() == "main" {
			return nil
		}
	}

	return []mg.Completion{{
		Query: `func main`,
		Title: `main() {...}`,
		Src: `
			func main() {
				$0
			}
		`,
	}}
}

func InitFuncSnippet(cx *CompletionCtx) []mg.Completion {
	if !cx.Scope.Is(FileScope) {
		return nil
	}

	for _, x := range cx.AstFile.Decls {
		x, ok := x.(*ast.FuncDecl)
		if ok && x.Name != nil && x.Name.String() == "init" {
			return nil
		}
	}

	return []mg.Completion{{
		Query: `func init`,
		Title: `init() {...}`,
		Src: `
			func init() {
				$0
			}
		`,
	}}
}

func FuncSnippet(cx *CompletionCtx) []mg.Completion {
	if cx.Scope.Is(FileScope) {
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

	if cx.Scope.Any(BlockScope, VarScope) {
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

func receiverName(typeName string) string {
	name := make([]rune, 0, 4)
	for _, r := range typeName {
		if len(name) == 0 || unicode.IsUpper(r) {
			name = append(name, unicode.ToLower(r))
		}
	}
	return string(name)
}

func MethodSnippet(cx *CompletionCtx) []mg.Completion {
	if cx.IsTestFile || !cx.Scope.Is(FileScope) {
		return nil
	}

	type field struct {
		nm  string
		typ string
	}
	fields := map[string]field{}
	types := []string{}

	for _, x := range cx.AstFile.Decls {
		switch x := x.(type) {
		case *ast.FuncDecl:
			if x.Recv == nil || len(x.Recv.List) == 0 {
				continue
			}

			r := x.Recv.List[0]
			if len(r.Names) == 0 {
				continue
			}

			name := ""
			if id := r.Names[0]; id != nil {
				name = id.String()
			}

			switch x := r.Type.(type) {
			case *ast.Ident:
				typ := x.String()
				fields[typ] = field{nm: name, typ: typ}
			case *ast.StarExpr:
				if id, ok := x.X.(*ast.Ident); ok {
					typ := id.String()
					fields[typ] = field{nm: name, typ: "*" + typ}
				}
			}
		case *ast.GenDecl:
			for _, spec := range x.Specs {
				spec, ok := spec.(*ast.TypeSpec)
				if ok && spec.Name != nil {
					types = append(types, spec.Name.String())
				}
			}
		}
	}

	cl := make([]mg.Completion, 0, len(types))
	for _, typ := range types {
		if f, ok := fields[typ]; ok {
			cl = append(cl, mg.Completion{
				Query: `func method ` + f.typ,
				Title: `(` + f.typ + `) method() {...}`,
				Src: `
					func (` + f.nm + ` ` + f.typ + `) ${1:name}($2)$3 {
						$0
					}
				`,
			})
		} else {
			nm := receiverName(typ)
			cl = append(cl, mg.Completion{
				Query: `func method ` + typ,
				Title: `(` + typ + `) method() {...}`,
				Src: `
					func (${1:` + nm + `} ${2:*` + typ + `}) ${3:name}($4)$5 {
						$0
					}
				`,
			})
		}
	}

	return cl
}
func (sf SnippetFuncs) name() {

}

func GenDeclSnippet(cx *CompletionCtx) []mg.Completion {
	if !cx.Scope.Is(FileScope) {
		return nil
	}
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
}

func MapSnippet(cx *CompletionCtx) []mg.Completion {
	if !cx.Scope.Any(VarScope, BlockScope) {
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

func TypeSnippet(cx *CompletionCtx) []mg.Completion {
	if !cx.Scope.Any(FileScope, BlockScope) {
		return nil
	}
	return []mg.Completion{
		{
			Query: `type struct`,
			Title: `struct {}`,
			Src: `
				type ${1:T} struct {
					${2:V}
				}
			`,
		},
		{
			Query: `type`,
			Title: `type T`,
			Src:   `type ${1:T} ${2:V}`,
		},
	}
}
