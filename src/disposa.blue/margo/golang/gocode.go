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
	"unicode"
)

var (
	gocodeClassTags = map[string]mg.CompletionTag{
		"const":   mg.ConstantTag,
		"func":    mg.FunctionTag,
		"package": mg.PackageTag,
		"import":  mg.PackageTag,
		"type":    mg.TypeTag,
		"var":     mg.VariableTag,
	}
)

type Gocode struct {
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

func (g *Gocode) Reduce(mx *mg.Ctx) *mg.State {
	st, gx := initGocodeReducer(mx, g)
	if gx == nil || !gx.query.completions {
		return st
	}

	candidates := gx.candidates()
	completions := make([]mg.Completion, 0, len(candidates))
	for _, v := range candidates {
		if c, ok := g.completion(mx, gx, v); ok {
			completions = append(completions, c)
		}
	}
	return st.AddCompletions(completions...)
}

func (g *Gocode) funcTitle(fx *ast.FuncType, buf *bytes.Buffer, decl string) string {
	// TODO: caching

	buf.Reset()
	fset := token.NewFileSet()

	buf.WriteString("func(")
	if fx.Params != nil {
		switch {
		case g.ShowFuncParams:
			g.printFields(buf, fset, fx.Params.List, true)
		case fx.Params.NumFields() != 0:
			buf.WriteString("…")
		}
	}
	buf.WriteString(")")

	if fl := fx.Results; fl != nil {
		buf.WriteString(" ")
		hasNames := g.ShowFuncResultNames && len(fl.List) != 0 && len(fl.List[0].Names) != 0
		if hasNames {
			buf.WriteString("(")
		}
		g.printFields(buf, fset, fl.List, g.ShowFuncResultNames)
		if hasNames {
			buf.WriteString(")")
		}
	}

	return buf.String()
}

func (g *Gocode) funcSrc(fx *ast.FuncType, buf *bytes.Buffer, v gocode.MargoCandidate, gx *gocodeCtx) string {
	// TODO: caching
	// TODO: only output the name, if we're in a call, assignment, etc. that takes a func

	outputArgs := true
	for _, c := range gx.src[gx.pos:] {
		if c == '(' {
			outputArgs = false
			break
		}
		r := rune(c)
		if !IsLetter(r) && !unicode.IsSpace(r) {
			break
		}
	}

	buf.Reset()
	buf.WriteString(v.Name)
	if outputArgs {
		buf.WriteString("(")
		for i, field := range fx.Params.List {
			if i > 0 {
				fmt.Fprint(buf, ", ")
			}
			for j, name := range field.Names {
				if j > 0 {
					fmt.Fprint(buf, ", ")
				}
				fmt.Fprintf(buf, "${%d:%s}", i+1, name)
			}
		}
		buf.WriteString(")")
	}
	buf.WriteString("${0}")
	return buf.String()
}

func (g *Gocode) printFields(w io.Writer, fset *token.FileSet, list []*ast.Field, printNames bool) {
	for i, field := range list {
		if i > 0 {
			fmt.Fprint(w, ", ")
		}
		if printNames {
			for j, name := range field.Names {
				if j > 0 {
					fmt.Fprint(w, ", ")
				}
				fmt.Fprint(w, name.String())
			}
			if len(field.Names) != 0 {
				fmt.Fprint(w, " ")
			}
		}
		printer.Fprint(w, fset, field.Type)
	}
}

func (g *Gocode) completion(mx *mg.Ctx, gx *gocodeCtx, v gocode.MargoCandidate) (c mg.Completion, ok bool) {
	buf := bytes.NewBuffer(nil)
	if v.Class.String() == "PANIC" {
		mx.Log.Printf("gocode panicked in '%s' at pos '%d'\n", gx.fn, gx.pos)
		return c, false
	}
	if !g.ProposeTests && g.matchTests(v) {
		return c, false
	}

	var fx *ast.FuncType
	if strings.HasPrefix(v.Type, "func(") {
		x, _ := parser.ParseExpr(v.Type)
		fx, _ = x.(*ast.FuncType)
	}

	c = mg.Completion{
		Query: g.compQuery(v),
		Tag:   g.compTag(v),
		Src:   g.compSrc(fx, buf, v, gx),
		Title: g.compTitle(fx, buf, v),
	}
	return c, true
}

func (g *Gocode) compQuery(v gocode.MargoCandidate) string {
	return v.Name
}

func (g *Gocode) compSrc(fx *ast.FuncType, buf *bytes.Buffer, v gocode.MargoCandidate, gx *gocodeCtx) string {
	if fx == nil {
		return v.Name
	}
	return g.funcSrc(fx, buf, v, gx)
}

func (g *Gocode) compTag(v gocode.MargoCandidate) mg.CompletionTag {
	if tag, ok := gocodeClassTags[v.Class.String()]; ok {
		return tag
	}
	return mg.UnknownTag
}

func (g *Gocode) compTitle(fx *ast.FuncType, buf *bytes.Buffer, v gocode.MargoCandidate) string {
	if fx != nil {
		return g.funcTitle(fx, buf, v.Type)
	}
	if v.Type == "" {
		return v.Class.String()
	}
	return v.Type
}

func (g *Gocode) matchTests(c gocode.MargoCandidate) bool {
	return strings.HasPrefix(c.Name, "Test") ||
		strings.HasPrefix(c.Name, "Benchmark") ||
		strings.HasPrefix(c.Name, "Example")
}

type gocodeCtx struct {
	Gocode
	cn    *CursorNode
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

func initGocodeReducer(mx *mg.Ctx, g *Gocode) (*mg.State, *gocodeCtx) {
	st := mx.State
	if !st.View.LangIs("go") {
		return st, nil
	}

	if cfg, ok := st.Config.(sublime.Config); ok {
		cfg = cfg.DisableGsComplete()
		if !g.AllowExplicitCompletions {
			cfg = cfg.InhibitExplicitCompletions()
		}
		if !g.AllowWordCompletions {
			cfg = cfg.InhibitWordCompletions()
		}
		st = st.SetConfig(cfg)
	}

	// TODO: use QueryCompletions.Pos when support is added
	_, tooltips := mx.Action.(mg.QueryTooltips)
	_, completions := mx.Action.(mg.QueryCompletions)
	if !completions && !tooltips {
		return st, nil
	}

	bctx := BuildContext(mx)
	src, _ := st.View.ReadAll()
	if len(src) == 0 {
		return st, nil
	}
	pos := clampSrcPos(src, st.View.Pos)
	pos = mg.BytePos(src, pos)

	cx := NewCompletionCtx(mx, src, pos)
	if cx.Scope.Any(PackageScope, FileScope) {
		return st, nil
	}
	cn := cx.CursorNode
	// don't do completion inside comments
	if cn.Comment != nil {
		return st, nil
	}
	// don't do completion inside strings unless it's an import
	if cn.ImportSpec == nil && cn.BasicLit != nil && cn.BasicLit.Kind == token.STRING {
		return st, nil
	}

	gx := &gocodeCtx{
		cn:   cn,
		fn:   st.View.Filename(),
		pos:  pos,
		src:  src,
		bctx: bctx,
		cfg: gocode.MargoConfig{
			GOROOT:             bctx.GOROOT,
			GOPATHS:            PathList(bctx.GOPATH),
			InstallSuffix:      g.InstallSuffix,
			ProposeBuiltins:    g.ProposeBuiltins,
			Autobuild:          g.Autobuild,
			UnimportedPackages: g.UnimportedPackages,
		},
	}
	gx.query.completions = completions
	gx.query.tooltips = tooltips
	return st, gx
}

func (gx *gocodeCtx) candidates() []gocode.MargoCandidate {
	if len(gx.src) == 0 {
		return nil
	}
	return gocode.Margo.Complete(gx.cfg, gx.src, gx.fn, gx.pos)
}

func clampSrcPos(src []byte, pos int) int {
	if pos < 0 {
		return 0
	}
	if pos > len(src) {
		return len(src) - 1
	}
	return pos
}
