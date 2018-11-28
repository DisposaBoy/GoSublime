package snippets

import (
	"go/ast"
	"margo.sh/golang/cursor"
	"margo.sh/mg"
	"unicode"
)

func receiverName(typeName string) string {
	name := make([]rune, 0, 4)
	for _, r := range typeName {
		if len(name) == 0 || unicode.IsUpper(r) {
			name = append(name, unicode.ToLower(r))
		}
	}
	return string(name)
}

func MethodSnippet(cx *cursor.CurCtx) []mg.Completion {
	if cx.Scope != cursor.FileScope && !cx.Scope.Is(cursor.FuncDeclScope) {
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
