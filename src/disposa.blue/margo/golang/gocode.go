package golang

import (
	"bytes"
	"disposa.blue/margo/golang/internal/gocode"
	"disposa.blue/margo/mg"
	"disposa.blue/margo/sublime"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"strings"
)

var (
	Gocode = GocodeConfig{}

	gocodeClassTags = map[string]mg.CompletionTag{
		"const":   mg.ConstantTag,
		"func":    mg.FunctionTag,
		"package": mg.PackageTag,
		"type":    mg.TypeTag,
		"var":     mg.VariableTag,
	}
)

type GocodeConfig struct {
	InstallSuffix            string
	ProposeBuiltins          bool
	ProposeTests             bool
	Autobuild                bool
	UnimportedPackages       bool
	AllowExplicitCompletions bool
	AllowWordCompletions     bool
	ShowFuncParams           bool
	ShowFuncResultNames      bool
}

func (g GocodeConfig) Completions(mx *mg.Ctx) *mg.State {
	mx, gx := initGocodeReducer(mx, g)
	if gx == nil || !gx.query.completions {
		return mx.State
	}

	// TODO: caching
	src, _ := mx.View.ReadAll()
	nn := ParseNearestNode(src, mx.View.Pos)
	if nn.CommentGroup != nil {
		return mx.State
	}
	if nn.BasicLit != nil && nn.BasicLit.Kind == token.STRING {
		return mx.State
	}

	candidates := gx.candidates()
	completions := make([]mg.Completion, 0, len(candidates))
	for _, v := range candidates {
		if c, ok := g.completion(gx, v); ok {
			completions = append(completions, c)
		}
	}
	return mx.AddCompletions(completions...)
}

func (g GocodeConfig) funcTitle(decl string) string {
	// TODO: caching

	x, _ := parser.ParseExpr(decl)
	f, _ := x.(*ast.FuncType)
	if f == nil {
		return decl
	}

	out := bytes.NewBuffer(nil)
	fset := token.NewFileSet()

	out.WriteString("func(")
	if f.Params != nil {
		switch {
		case g.ShowFuncParams:
			g.printFields(out, fset, f.Params.List, true)
		case f.Params.NumFields() != 0:
			out.WriteString("…")
		}
	}
	out.WriteString(")")

	if fl := f.Results; fl != nil {
		out.WriteString(" ")
		hasNames := g.ShowFuncResultNames && len(fl.List) != 0 && len(fl.List[0].Names) != 0
		if hasNames {
			out.WriteString("(")
		}
		g.printFields(out, fset, fl.List, g.ShowFuncResultNames)
		if hasNames {
			out.WriteString(")")
		}
	}

	return out.String()
}

func (g GocodeConfig) printFields(out io.Writer, fset *token.FileSet, list []*ast.Field, printNames bool) {
	for i, field := range list {
		if i > 0 {
			fmt.Fprint(out, ", ")
		}
		if printNames {
			for j, name := range field.Names {
				if j > 0 {
					fmt.Fprint(out, ", ")
				}
				fmt.Fprint(out, name.String())
			}
			if len(field.Names) != 0 {
				fmt.Fprint(out, " ")
			}
		}
		printer.Fprint(out, fset, field.Type)
	}
}

func (g GocodeConfig) completion(gx *gocodeCtx, v gocode.MargoCandidate) (c mg.Completion, ok bool) {
	vclass := v.Class.String()
	if vclass == "PANIC" {
		mg.Log.Printf("gocode panicked in '%s' at pos '%d'\n", gx.fn, gx.pos)
		return c, false
	}
	if !g.ProposeTests && g.matchTests(v) {
		return c, false
	}

	c = mg.Completion{
		Query: v.Name,
		Src:   v.Name,
		Tag:   gocodeClassTags[vclass],
	}
	switch {
	case v.Type == "":
		c.Title = vclass
	case strings.HasPrefix(v.Type, "func("):
		c.Title = g.funcTitle(v.Type)
	default:
		c.Title = v.Type
	}
	return c, true
}

func (g GocodeConfig) matchTests(c gocode.MargoCandidate) bool {
	return strings.HasPrefix(c.Name, "Test") ||
		strings.HasPrefix(c.Name, "Benchmark") ||
		strings.HasPrefix(c.Name, "Example")
}

type gocodeCtx struct {
	GocodeConfig
	fn    string
	src   []byte
	pos   int
	bctx  *build.Context
	cfg   gocode.MargoConfig
	query struct {
		completions bool
		tooltips    bool
	}
}

func initGocodeReducer(mx *mg.Ctx, gc GocodeConfig) (*mg.Ctx, *gocodeCtx) {
	// TODO: ignore comments and strings

	if cfg, ok := mx.Config.(sublime.Config); ok {
		cfg = cfg.DisableGsComplete()
		if !gc.AllowExplicitCompletions {
			cfg = cfg.InhibitExplicitCompletions()
		}
		if !gc.AllowWordCompletions {
			cfg = cfg.InhibitWordCompletions()
		}
		mx = mx.Copy(func(mx *mg.Ctx) {
			mx.State = mx.SetConfig(cfg)
		})
	}

	if !mx.View.LangIs("go") {
		return mx, nil
	}

	qry := gocodeCtx{}.query
	_, qry.completions = mx.Action.(mg.QueryCompletions)
	_, qry.tooltips = mx.Action.(mg.QueryTooltips)
	if !qry.completions && !qry.tooltips {
		return mx, nil
	}

	bctx := BuildContext(mx.Env)
	src, _ := mx.View.ReadAll()
	return mx, &gocodeCtx{
		query: qry,
		fn:    mx.View.Filename(),
		src:   src,
		pos:   mx.View.Pos,
		bctx:  bctx,
		cfg: gocode.MargoConfig{
			GOROOT:             bctx.GOROOT,
			GOPATHS:            PathList(bctx.GOPATH),
			InstallSuffix:      gc.InstallSuffix,
			ProposeBuiltins:    gc.ProposeBuiltins,
			Autobuild:          gc.Autobuild,
			UnimportedPackages: gc.UnimportedPackages,
		},
	}
}

func (gx *gocodeCtx) setPos(pos int) {
	switch {
	case pos < 0:
		gx.pos = 0
	case pos > len(gx.src):
		gx.pos = len(gx.src)
	default:
		gx.pos = pos
	}
}

func (gx *gocodeCtx) candidates() []gocode.MargoCandidate {
	if len(gx.src) == 0 {
		return nil
	}
	return gocode.Margo.Complete(gx.cfg, gx.src, gx.fn, gx.pos)
}

// func (gx *gocodeCtx) calltips() []gocode.MargoCandidate {
// 	id, fset, af := identAtOffset(gx.src, offset)
// 	if id != nil {
// 		cp := fset.Position(id.End())
// 		if cp.IsValid() {
// 			line := offsetLine(fset, af, offset)
// 			gx.setPos(cp.Offset)
// 			cl := gx.candidates()

// 			if (cp.Line == line || line == 0) && len(cl) > 0 {
// 				for i, c := range cl {
// 					if strings.EqualFold(id.Name, c.Name) {
// 						return cl[i : i+1]
// 					}
// 				}
// 			}
// 		}
// 	}

// 	return []gocode.MargoCandidate{}
// }

func identAtOffset(src []byte, offset int) (*ast.Ident, *token.FileSet, *ast.File) {
	fset := token.NewFileSet()
	af, _ := parser.ParseFile(fset, "<stdin>", src, 0)

	if af == nil {
		return nil, fset, nil
	}
	tf := fset.File(af.Pos())
	if af == nil {
		return nil, fset, af
	}

	nn := &NearestNode{Pos: tf.Pos(offset)}
	nn.ScanFile(af)
	var id *ast.Ident
	if nn.CallExpr != nil && nn.CallExpr.Fun != nil {
		switch v := nn.CallExpr.Fun.(type) {
		case *ast.Ident:
			id = v
		case *ast.SelectorExpr:
			id = v.Sel
		}
	}
	return id, fset, af
}

func offsetLine(fset *token.FileSet, af *ast.File, offset int) (line int) {
	defer func() {
		if err := recover(); err != nil {
			line = 0
		}
	}()
	return fset.File(af.Pos()).Position(token.Pos(offset)).Line
}
