package golang

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"margo.sh/golang/internal/gocode"
	"margo.sh/mg"
	"margo.sh/mgpf"
	"margo.sh/sublime"
	"strings"
	"time"
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

type gocodeReq struct {
	g   *Gocode
	mx  *mg.Ctx
	st  *mg.State
	gx  *gocodeCtx
	res chan *mg.State
}

func (gr *gocodeReq) reduce() *mg.State {
	candidates := gr.gx.candidates()
	completions := make([]mg.Completion, 0, len(candidates))
	for _, v := range candidates {
		if c, ok := gr.g.completion(gr.mx, gr.gx, v); ok {
			completions = append(completions, c)
		}
	}
	return gr.st.AddCompletions(completions...)
}

type Gocode struct {
	mg.ReducerType

	InstallSuffix            string
	ProposeBuiltins          bool
	ProposeTests             bool
	Autobuild                bool
	UnimportedPackages       bool
	AllowExplicitCompletions bool
	AllowWordCompletions     bool
	ShowFuncParams           bool
	ShowFuncResultNames      bool
	Debug                    bool

	reqs chan gocodeReq
}

func (g *Gocode) ReducerConfig(mx *mg.Ctx) mg.EditorConfig {
	cfg, ok := mx.Config.(sublime.Config)
	if !ok {
		return nil
	}

	// ST might query the GoSublime plugin first, so we must always disable it
	cfg = cfg.DisableGsComplete()
	// but we don't want to affect editor completions in non-go files
	if !g.ReducerCond(mx) {
		return cfg
	}

	if !g.AllowExplicitCompletions {
		cfg = cfg.InhibitExplicitCompletions()
	}
	if !g.AllowWordCompletions {
		cfg = cfg.InhibitWordCompletions()
	}
	return cfg
}

func (g *Gocode) ReducerCond(mx *mg.Ctx) bool {
	return mx.ActionIs(mg.QueryCompletions{}) && mx.LangIs(mg.Go)
}

func (g *Gocode) ReducerMount(mx *mg.Ctx) {
	g.reqs = make(chan gocodeReq)
	go func() {
		for gr := range g.reqs {
			gr.res <- gr.reduce()
		}
	}()
}

func (g *Gocode) ReducerUnmount(mx *mg.Ctx) {
	close(g.reqs)
}

func (g *Gocode) Reduce(mx *mg.Ctx) *mg.State {
	start := time.Now()
	st, gx := initGocodeReducer(mx, *g)
	if gx == nil {
		return st
	}

	qTimeout := 100 * time.Millisecond
	gr := gocodeReq{
		g:   g,
		mx:  mx,
		st:  st,
		gx:  gx,
		res: make(chan *mg.State, 1),
	}
	select {
	case g.reqs <- gr:
	case <-time.After(qTimeout):
		mx.Log.Println("gocode didn't accept the request after", mgpf.D(time.Since(start)))
		return st
	}

	pTimeout := 150 * time.Millisecond
	if d := qTimeout - time.Since(start); d > 0 {
		pTimeout += d
	}

	select {
	case st := <-gr.res:
		return st
	case <-time.After(pTimeout):
		go func() {
			<-gr.res
			mx.Log.Println("gocode eventually responded after", mgpf.Since(start))
		}()

		mx.Log.Println("gocode didn't respond after", mgpf.D(pTimeout), "taking", mgpf.Since(start))
		return st
	}
}

func (g Gocode) funcTitle(fx *ast.FuncType, buf *bytes.Buffer, decl string) string {
	// TODO: caching

	buf.Reset()
	fset := token.NewFileSet()

	buf.WriteString("func(")
	if fx.Params != nil {
		switch {
		case g.ShowFuncParams:
			printFields(buf, fset, fx.Params.List, true)
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
		printFields(buf, fset, fl.List, g.ShowFuncResultNames)
		if hasNames {
			buf.WriteString(")")
		}
	}

	return buf.String()
}

func (g Gocode) funcSrc(fx *ast.FuncType, buf *bytes.Buffer, v gocode.MargoCandidate, gx *gocodeCtx) string {
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
		pos := 0
		for _, field := range fx.Params.List {
			for _, name := range field.Names {
				pos++
				if pos > 1 {
					buf.WriteString(", ")
				}
				fmt.Fprintf(buf, "${%d:%s}", pos, name)
			}
		}
		buf.WriteString(")")
	}
	buf.WriteString("${0}")
	return buf.String()
}

func printFields(w io.Writer, fset *token.FileSet, list []*ast.Field, printNames bool) {
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

func (g Gocode) completion(mx *mg.Ctx, gx *gocodeCtx, v gocode.MargoCandidate) (c mg.Completion, ok bool) {
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

func (g Gocode) compQuery(v gocode.MargoCandidate) string {
	return v.Name
}

func (g Gocode) compSrc(fx *ast.FuncType, buf *bytes.Buffer, v gocode.MargoCandidate, gx *gocodeCtx) string {
	if fx == nil {
		return v.Name
	}
	return g.funcSrc(fx, buf, v, gx)
}

func (g Gocode) compTag(v gocode.MargoCandidate) mg.CompletionTag {
	if tag, ok := gocodeClassTags[v.Class.String()]; ok {
		return tag
	}
	return mg.UnknownTag
}

func (g Gocode) compTitle(fx *ast.FuncType, buf *bytes.Buffer, v gocode.MargoCandidate) string {
	if fx != nil {
		return g.funcTitle(fx, buf, v.Type)
	}
	if v.Type == "" {
		return v.Class.String()
	}
	return v.Type
}

func (g Gocode) matchTests(c gocode.MargoCandidate) bool {
	return strings.HasPrefix(c.Name, "Test") ||
		strings.HasPrefix(c.Name, "Benchmark") ||
		strings.HasPrefix(c.Name, "Example")
}

type gocodeCtx struct {
	Gocode
	cn   *CursorNode
	fn   string
	src  []byte
	pos  int
	bctx *build.Context
	cfg  gocode.MargoConfig
}

func initGocodeReducer(mx *mg.Ctx, g Gocode) (*mg.State, *gocodeCtx) {
	st := mx.State
	bctx := BuildContext(mx)
	src, _ := st.View.ReadAll()
	if len(src) == 0 {
		return st, nil
	}
	pos := clampSrcPos(src, st.View.Pos)

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
			Debug:              g.Debug,
		},
	}
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
