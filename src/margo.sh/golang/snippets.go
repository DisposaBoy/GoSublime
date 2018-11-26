package golang

import (
	"go/ast"
	"margo.sh/mg"
	yotsuba "margo.sh/why_would_you_make_yotsuba_cry"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

var (
	Snippets = SnippetFuncs(
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
		ImportPathSnippet,
	)

	pkgDirNamePat = regexp.MustCompile(`(\w+)\W*$`)
)

type SnippetFunc func(*CompletionCtx) []mg.Completion

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
	c.Src = Dedent(c.Src)
	if c.Tag == "" {
		c.Tag = mg.SnippetTag
	}
}

func PackageNameSnippet(cx *CompletionCtx) []mg.Completion {
	if cx.PkgName != NilPkgName || cx.Scope != PackageScope {
		return nil
	}

	var cl []mg.Completion
	seen := map[string]bool{}
	add := func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		cl = append(cl, mg.Completion{
			Query: `package ` + name,
			Src: `
				package ` + name + `

				$0
			`,
		})
	}

	dir := cx.View.Dir()
	pkg, _ := BuildContext(cx.Ctx).ImportDir(dir, 0)
	if pkg != nil && pkg.Name != "" {
		add(pkg.Name)
	} else {
		add(pkgDirNamePat.FindString(dir))
	}
	add("main")

	return cl
}

func MainFuncSnippet(cx *CompletionCtx) []mg.Completion {
	if cx.Scope != FileScope || cx.PkgName != "main" {
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
	if cx.Scope != FileScope {
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
	if cx.Scope == FileScope || cx.Scope.Is(FuncDeclScope) {
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

	if cx.Scope.Is(BlockScope, VarScope) {
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
	if cx.Scope != FileScope && !cx.Scope.Is(FuncDeclScope) {
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
				if ok && spec.Name != nil && spec.Name.Name != "_" {
					types = append(types, spec.Name.Name)
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
					func (${1:` + nm + `} ${2:*}` + typ + `) ${3:name}($4)$5 {
						$0
					}
				`,
			})
		}
	}

	return cl
}

func GenDeclSnippet(cx *CompletionCtx) []mg.Completion {
	switch cx.Scope {
	case BlockScope:
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
	case FileScope:
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

func MapSnippet(cx *CompletionCtx) []mg.Completion {
	if !cx.Scope.Is(ExprScope) {
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
	if cs := cx.Scope; cs != FileScope && cs != BlockScope && !cs.Is(TypeDeclScope) {
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

func AppendSnippet(cx *CompletionCtx) []mg.Completion {
	if !cx.Scope.Is(ExprScope) {
		return nil
	}

	if !cx.Scope.Is(AssignmentScope) {
		return []mg.Completion{
			mg.Completion{
				Query: `append`,
				Title: `append(S, E)`,
				Src:   `append(${1}, ${2})$0`,
			},
		}
	}

	var asn *ast.AssignStmt
	if !cx.Set(&asn) || len(asn.Lhs) != 1 || len(asn.Rhs) > 1 {
		return nil
	}
	id, _ := asn.Lhs[0].(*ast.Ident)
	if id == nil {
		return nil
	}

	return []mg.Completion{
		mg.Completion{
			Query: `append`,
			Title: `append(` + id.Name + `, E)`,
			Src:   `append(${1:` + id.Name + `}, ${2})$0`,
		},
	}
}

func DocSnippet(cx *CompletionCtx) []mg.Completion {
	if cx.Doc == nil {
		return nil
	}

	var ids []*ast.Ident
	var addNames func(n ast.Node)
	addFieldNames := func(fl *ast.FieldList) {
		if fl == nil {
			return
		}
		for _, f := range fl.List {
			ids = append(ids, f.Names...)
			addNames(f.Type)
		}
	}
	addNames = func(n ast.Node) {
		if yotsuba.IsNil(n) {
			return
		}

		switch x := n.(type) {
		case *ast.GenDecl:
			for _, spec := range x.Specs {
				addNames(spec)
			}
		case *ast.SelectorExpr:
			addNames(x.Sel)
			addNames(x.X)
		case *ast.Ident:
			ids = append(ids, x)
		case *ast.File:
			ids = append(ids, x.Name)
		case *ast.FieldList:
			addFieldNames(x)
		case *ast.Field:
			ids = append(ids, x.Names...)
		case *ast.TypeSpec:
			ids = append(ids, x.Name)
		case *ast.FuncDecl:
			ids = append(ids, x.Name)
			addFieldNames(x.Recv)
			if t := x.Type; t != nil {
				addFieldNames(t.Params)
				addFieldNames(t.Results)
			}
		case *ast.ValueSpec:
			addNames(x.Type)
			ids = append(ids, x.Names...)
		}
	}
	addNames(cx.Doc.Node)

	seen := map[string]bool{}
	cl := make([]mg.Completion, 0, len(ids))
	for _, id := range ids {
		if id == nil || id.Name == "_" || seen[id.Name] {
			continue
		}
		seen[id.Name] = true
		cl = append(cl, mg.Completion{
			Query: id.Name,
			Src:   id.Name + ` $0`,
		})
	}
	return cl
}

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
