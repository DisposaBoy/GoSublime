package golang

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"kuroku.io/margocode/suggest"
	"margo.sh/htm"
	"margo.sh/mg"
	"margo.sh/mgutil"
	"margo.sh/sublime"
	"strings"
	"unicode"
)

const (
	calltipOpenTag  = "⎨"
	calltipCloseTag = "⎬"
)

type gocodeCtAct struct {
	mg.ActionType
	mx     *mg.Ctx
	status string
}

type GocodeCalltips struct {
	mg.ReducerType

	// The following fields are deprecated

	// This field is ignored, see MarGocodeCtl.ImporterMode
	Source bool
	// Consider using MarGocodeCtl.Debug instead, it has more useful output
	Debug bool

	q      *mgutil.ChanQ
	status string
	hud    htm.Element
}

func (gc *GocodeCalltips) RCond(mx *mg.Ctx) bool {
	return mx.LangIs(mg.Go)
}

func (gc *GocodeCalltips) RMount(mx *mg.Ctx) {
	gc.q = mgutil.NewChanQ(1)
	go gc.processer()
}

func (gc *GocodeCalltips) RUnmount(mx *mg.Ctx) {
	gc.q.Close()
}

func (gc *GocodeCalltips) Reduce(mx *mg.Ctx) *mg.State {
	st := mx.State
	if cfg, ok := st.Config.(sublime.Config); ok {
		st = st.SetConfig(cfg.DisableCalltips())
	}

	switch act := mx.Action.(type) {
	case mg.ViewPosChanged, mg.ViewActivated:
		gc.q.Put(gocodeCtAct{mx: mx, status: gc.status})
	case gocodeCtAct:
		s := act.status
		gc.status = s
		i := strings.Index(s, calltipOpenTag)
		j := strings.Index(s, calltipCloseTag)
		switch {
		case i >= 0 && j > i:
			gc.hud = htm.Span(nil,
				htm.Text(s[:i]),
				htm.HighlightText(s[i+len(calltipOpenTag):j]),
				htm.Text(s[j+len(calltipOpenTag):]),
			)
		case gc.status == "":
			gc.hud = nil
		default:
			gc.hud = htm.Text(s)
		}
	}

	if gc.status != "" {
		st = st.AddStatus(gc.status)
	}
	if gc.hud != nil {
		st = st.AddHUD(htm.Text("Calltips"), gc.hud)
	}
	return st
}

func (gc *GocodeCalltips) processer() {
	for a := range gc.q.C() {
		gc.process(a.(gocodeCtAct))
	}
}

func (gc *GocodeCalltips) process(act gocodeCtAct) {
	defer func() { recover() }()

	if s := gc.processStatus(act); s != act.status {
		act.mx.Store.Dispatch(gocodeCtAct{status: s})
	}
}

func (gc *GocodeCalltips) processStatus(act gocodeCtAct) string {
	mx := act.mx
	src, srcPos := mx.View.SrcPos()
	if len(src) == 0 {
		return ""
	}

	cx := NewCursorCtx(mx, src, srcPos)
	tf := cx.TokenFile
	tokPos := tf.Pos(srcPos)
	call, assign := gc.findCallExpr(cx.Nodes, tokPos)
	if call == nil {
		return ""
	}

	ident := gc.exprIdent(call.Fun)
	if ident == nil {
		return ""
	}

	fxName := ident.String()
	candidate, ok := gc.candidate(mx, src, tf.Position(ident.End()).Offset, fxName)
	if !ok {
		return ""
	}

	expr, _ := parser.ParseExpr(candidate.Type)
	fx, _ := expr.(*ast.FuncType)
	if fx == nil {
		return ""
	}

	var highlight ast.Node
	switch {
	case call.Lparen < tokPos && tokPos <= call.Rparen:
		i := gc.selectedFieldExpr(tf.Offset, src, srcPos, call.Args)
		highlight = gc.selectedFieldName(fx.Params, i)
	case assign != nil:
		i := gc.selectedFieldExpr(tf.Offset, src, srcPos, assign.Lhs)
		highlight = gc.selectedFieldName(fx.Results, i)
	}

	return gc.funcSrc(fx, fxName, highlight)
}

func (gc *GocodeCalltips) findCallExpr(nodes []ast.Node, pos token.Pos) (*ast.CallExpr, *ast.AssignStmt) {
	var assign *ast.AssignStmt
	var call, callCandidate *ast.CallExpr
out:
	for i := len(nodes) - 1; i >= 0; i-- {
		switch x := nodes[i].(type) {
		case *ast.BlockStmt:
			break out
		case *ast.AssignStmt:
			assign = x
		case *ast.CallExpr:
			// we found a CallExpr, but it's not necessarily the right one.
			// in `funcF(fun|cG())` this will match funcG, but we want funcF
			// so we track of the first CallExpr but keep searching until we find one
			// whose left paren is before the cursor
			if callCandidate == nil {
				callCandidate = x
			}
			if x.Lparen < pos {
				call = x
				break out
			}
		}
	}

	switch {
	case call != nil:
		return call, nil
	case callCandidate != nil:
		return callCandidate, nil
	case assign != nil && len(assign.Rhs) == 1:
		if call, ok := assign.Rhs[0].(*ast.CallExpr); ok {
			return call, assign
		}
	}
	return nil, nil
}

func (gc *GocodeCalltips) funcSrc(fx *ast.FuncType, funcName string, highlight ast.Node) string {
	fset := token.NewFileSet()
	buf := &bytes.Buffer{}

	buf.WriteString("func ")
	buf.WriteString(funcName)

	var params []*ast.Field
	if p := fx.Params; p != nil {
		params = p.List
	}
	fieldPrinter{
		fset:      fset,
		fields:    params,
		buf:       buf,
		parens:    true,
		names:     true,
		types:     true,
		highlight: highlight,
	}.print()

	if p := fx.Results; p != nil {
		buf.WriteByte(' ')
		fieldPrinter{
			fset:      fset,
			fields:    p.List,
			buf:       buf,
			parens:    len(p.List) != 0 && len(p.List[0].Names) != 0,
			names:     true,
			types:     true,
			highlight: highlight,
		}.print()
	}

	return buf.String()
}

func (gc *GocodeCalltips) selectedFieldName(fl *ast.FieldList, fieldIndex int) ast.Node {
	if fl == nil || len(fl.List) == 0 {
		return nil
	}

	index := 0
	for _, field := range fl.List {
		if len(field.Names) == 0 {
			if index == fieldIndex {
				return field
			}
			index++
			continue
		}

		for _, id := range field.Names {
			if index == fieldIndex {
				return id
			}
			index++
		}
	}

	f := fl.List[len(fl.List)-1]
	if _, ok := f.Type.(*ast.Ellipsis); ok && len(f.Names) == 1 {
		return f.Names[0]
	}

	return nil
}

func (gc *GocodeCalltips) selectedFieldExpr(offset func(token.Pos) int, src []byte, pos int, fields []ast.Expr) int {
	for i, a := range fields {
		np := mgutil.RepositionLeft(src, offset(a.Pos()), unicode.IsSpace)
		ne := mgutil.RepositionRight(src, offset(a.End()), unicode.IsSpace)
		if np <= pos && pos <= ne {
			return i
		}
	}
	// in most cases we're after a comma,
	// so choose the next field (that doesn't exist yet)
	return len(fields)
}

func (gc *GocodeCalltips) candidate(mx *mg.Ctx, src []byte, pos int, funcName string) (candidate suggest.Candidate, ok bool) {
	if pos < 0 || pos >= len(src) {
		return candidate, false
	}

	gsu := mctl.newGcSuggest(mx)
	gsu.suggestDebug = gc.Debug
	sugg := gsu.suggestions(mx, src, pos)
	for _, c := range sugg.candidates {
		if !strings.HasPrefix(c.Type, "func(") {
			continue
		}
		switch {
		case funcName == c.Name:
			return c, true
		case strings.EqualFold(funcName, c.Name):
			candidate = c
		}
	}
	return candidate, candidate != suggest.Candidate{}
}

func (gc *GocodeCalltips) exprIdent(x ast.Expr) *ast.Ident {
	switch x := x.(type) {
	case *ast.Ident:
		return x
	case *ast.SelectorExpr:
		return x.Sel
	}
	return nil
}

type fieldPrinter struct {
	fset      *token.FileSet
	fields    []*ast.Field
	buf       *bytes.Buffer
	names     bool
	types     bool
	parens    bool
	highlight ast.Node
}

func (p fieldPrinter) print() {
	w := p.buf
	if p.parens {
		w.WriteByte('(')
	}

	hlId, _ := p.highlight.(*ast.Ident)
	hlField, _ := p.highlight.(*ast.Field)
	hlWriteOpen := func() { w.WriteString(calltipOpenTag) }
	hlWriteClose := func() { w.WriteString(calltipCloseTag) }

	for i, f := range p.fields {
		if i > 0 {
			w.WriteString(", ")
		}

		if f == hlField {
			hlWriteOpen()
		}

		var names []*ast.Ident
		if p.names {
			names = f.Names
		}
		for j, id := range names {
			if j > 0 {
				w.WriteString(", ")
			}
			if hlId == id {
				hlWriteOpen()
			}
			w.WriteString(id.String())

			if hlId == id && j < len(names)-1 {
				hlWriteClose()
			}
		}

		if p.types {
			if len(names) != 0 {
				w.WriteByte(' ')
			}
			printer.Fprint(w, p.fset, f.Type)
		}

		if l := names; f == hlField || (len(l) > 0 && l[len(l)-1] == hlId) {
			hlWriteClose()
		}
	}

	if p.parens {
		w.WriteByte(')')
	}
}
