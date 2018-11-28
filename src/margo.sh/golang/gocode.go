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
	"kuroku.io/margocode/suggest"
	"margo.sh/mg"
	"margo.sh/mgpf"
	"margo.sh/sublime"
	"os"
	"path"
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

type suggestions struct {
	candidates []suggest.Candidate
	unimported impSpec
}

func (gr *gocodeReq) addUnimportedPkg(st *mg.State, p impSpec) *mg.State {
	if !gr.gx.gsu.cfg.AddUnimportedPackages {
		return st
	}
	if p.Path == "" {
		return st
	}

	src, _ := st.View.ReadAll()
	if len(src) == 0 {
		return st
	}

	if p.Name == path.Base(p.Path) {
		p.Name = ""
	}

	s, ok := updateImports(st.View.Filename(), src, impSpecList{p}, nil)
	if ok {
		st = st.SetViewSrc(s)
	}

	return st
}

func (gr *gocodeReq) reduce() *mg.State {
	sugg := gr.gx.suggestions()
	completions := make([]mg.Completion, 0, len(sugg.candidates))

	st := gr.st
	if len(sugg.candidates) != 0 {
		st = gr.addUnimportedPkg(st, sugg.unimported)
	}

	gr.mx.Profile.Push("gocodeReq.finalize").Pop()
	for _, v := range sugg.candidates {
		if c, ok := gr.g.completion(gr.mx, gr.gx, v); ok {
			completions = append(completions, c)
		}
	}

	return st.AddCompletions(completions...)
}

type Gocode struct {
	mg.ReducerType

	AllowExplicitCompletions bool
	AllowWordCompletions     bool
	ShowFuncParams           bool
	ShowFuncResultNames      bool

	// The following fields are deprecated

	// Consider using MarGocodeCtl.Debug instead, it has more useful output
	Debug bool
	// This field is ignored, see MarGocodeCtl.ImporterMode
	Source bool
	// This field is ignored, see MarGocodeCtl.NoBuiltins
	ProposeBuiltins bool
	// This field is ignored, see MarGocodeCtl.ProposeTests
	ProposeTests bool
	// This field is ignored, see MarGocodeCtl.ImporterMode
	Autobuild bool
	// This field is ignored, See MarGocodeCtl.NoUnimportedPackages
	UnimportedPackages bool

	reqs chan gocodeReq
}

func (g *Gocode) RConfig(mx *mg.Ctx) mg.EditorConfig {
	cfg, ok := mx.Config.(sublime.Config)
	if !ok {
		return nil
	}

	// ST might query the GoSublime plugin first, so we must always disable it
	cfg = cfg.DisableGsComplete()
	// but we don't want to affect editor completions in non-go files
	if !g.RCond(mx) {
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

func (g *Gocode) RCond(mx *mg.Ctx) bool {
	return mx.ActionIs(mg.QueryCompletions{}) && mx.LangIs(mg.Go)
}

func (g *Gocode) RMount(mx *mg.Ctx) {
	g.reqs = make(chan gocodeReq)
	go func() {
		for gr := range g.reqs {
			gr.res <- gr.reduce()
		}
	}()
}

func (g *Gocode) RUnmount(mx *mg.Ctx) {
	close(g.reqs)
}

func (g *Gocode) Reduce(mx *mg.Ctx) *mg.State {
	start := time.Now()

	gx := initGocodeReducer(mx, *g)
	st := mx.State
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

			if g.Debug {
				opts := mgpf.DefaultPrintOpts
				opts.MinDuration = 3 * time.Millisecond
				mx.Profile.Fprint(os.Stderr, &opts)
			}
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

func (g Gocode) funcSrc(fx *ast.FuncType, buf *bytes.Buffer, v suggest.Candidate, gx *gocodeCtx) string {
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

func (g Gocode) completion(mx *mg.Ctx, gx *gocodeCtx, v suggest.Candidate) (c mg.Completion, ok bool) {
	buf := bytes.NewBuffer(nil)
	if v.Class == "PANIC" {
		mx.Log.Printf("gocode panicked in '%s' at pos '%d'\n", gx.fn, gx.pos)
		return c, false
	}
	if !gx.gsu.cfg.ProposeTests && g.matchTests(v) {
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

func (g Gocode) compQuery(v suggest.Candidate) string {
	return v.Name
}

func (g Gocode) compSrc(fx *ast.FuncType, buf *bytes.Buffer, v suggest.Candidate, gx *gocodeCtx) string {
	if fx == nil {
		return v.Name
	}
	return g.funcSrc(fx, buf, v, gx)
}

func (g Gocode) compTag(v suggest.Candidate) mg.CompletionTag {
	if tag, ok := gocodeClassTags[v.Class]; ok {
		return tag
	}
	return mg.UnknownTag
}

func (g Gocode) compTitle(fx *ast.FuncType, buf *bytes.Buffer, v suggest.Candidate) string {
	if fx != nil {
		return g.funcTitle(fx, buf, v.Type)
	}
	if v.Type == "" {
		return v.Class
	}
	return v.Type
}

func (g Gocode) matchTests(c suggest.Candidate) bool {
	if !strings.HasPrefix(c.Type, "func(") {
		return false
	}
	return strings.HasPrefix(c.Name, "Test") ||
		strings.HasPrefix(c.Name, "Benchmark") ||
		strings.HasPrefix(c.Name, "Example")
}

type gocodeCtx struct {
	Gocode
	*CursorCtx
	gsu  *gcSuggest
	mx   *mg.Ctx
	fn   string
	src  []byte
	pos  int
	bctx *build.Context
}

func initGocodeReducer(mx *mg.Ctx, g Gocode) *gocodeCtx {
	// TODO: simplify and get rid of this func, it's only used once

	src, pos := mx.View.SrcPos()
	if len(src) == 0 {
		return nil
	}

	cx := NewCursorCtx(mx, src, pos)
	if cx.Scope.Is(
		PackageScope,
		FileScope,
		ImportScope,
		StringScope,
		CommentScope,
		FuncDeclScope,
		TypeDeclScope,
	) {
		return nil
	}

	gsu := mctl.newGcSuggest(mx)
	gsu.suggestDebug = g.Debug
	return &gocodeCtx{
		mx:        mx,
		CursorCtx: cx,
		gsu:       gsu,
		fn:        mx.View.Filename(),
		pos:       pos,
		src:       src,
		bctx:      BuildContext(mx),
	}
}

func (gx *gocodeCtx) suggestions() suggestions {
	if len(gx.src) == 0 {
		return suggestions{}
	}
	return gx.gsu.suggestions(gx.mx, gx.src, gx.pos)
}
