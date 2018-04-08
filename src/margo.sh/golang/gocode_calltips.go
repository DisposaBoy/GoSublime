package golang

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"margo.sh/golang/internal/gocode"
	"margo.sh/mg"
	"margo.sh/sublime"
	"strings"
)

type GocodeCalltips struct{}

func (gc *GocodeCalltips) Reduce(mx *mg.Ctx) *mg.State {
	st := mx.State
	if !mx.LangIs("go") {
		return st
	}
	if cfg, ok := st.Config.(sublime.Config); ok {
		st = st.SetConfig(cfg.DisableCalltips())
	}

	src, _ := mx.View.ReadAll()
	if len(src) == 0 {
		return st
	}

	type key struct{ hash string }
	k := key{mg.SrcHash(src)}

	if mx.ActionIs(mg.ViewPosChanged{}, mg.ViewActivated{}) {
		gc.process(mx, k, src)
	}

	if s, _ := mx.Store.Get(k).(string); s != "" {
		return st.AddStatus(s)
	}
	return st
}

func (gc *GocodeCalltips) process(mx *mg.Ctx, key interface{}, src []byte) {
	mx.Store.Del(key)

	srcPos := clampSrcPos(src, mx.View.Pos)
	srcPos = mg.BytePos(src, srcPos)
	cn := ParseCursorNode(nil, src, srcPos)
	tpos := cn.TokenFile.Pos(srcPos)
	var call *ast.CallExpr
	for i := len(cn.Nodes) - 1; i >= 0; i-- {
		x, ok := cn.Nodes[i].(*ast.CallExpr)
		if !ok {
			continue
		}

		// we found a CallExpr, but it's not necessarily the right one.
		// in `funcF(fun|cG())` this will match funcG, but we want funcF
		// so we track of the first CallExpr but keep searching until we find one
		// whose left paren is before the cursor
		if call == nil {
			call = x
		}
		if x.Lparen < tpos {
			call = x
			break
		}
	}
	if call == nil {
		return
	}

	ident := gc.exprIdent(call.Fun)
	if ident == nil {
		return
	}

	funcName := ident.String()
	idPos := cn.TokenFile.Position(ident.End()).Offset
	candidate, ok := gc.candidate(mx, src, idPos, funcName)
	if !ok {
		return
	}

	x, _ := parser.ParseExpr(candidate.Type)
	fx, _ := x.(*ast.FuncType)
	if fx == nil {
		return
	}

	mx.Store.Put(key, gc.funcSrc(fx, gc.argPos(call, tpos), funcName))
}

func (gc *GocodeCalltips) funcSrc(fx *ast.FuncType, argPos int, funcName string) string {
	fset := token.NewFileSet()
	buf := &bytes.Buffer{}
	buf.WriteString("func ")
	buf.WriteString(funcName)
	buf.WriteString("(")
	fieldPos := 0
	for inField, field := range fx.Params.List {
		for _, name := range field.Names {
			if fieldPos > 0 {
				buf.WriteString(", ")
			}
			fieldPos++

			if inField == argPos {
				buf.WriteString("⎨")
			}
			buf.WriteString(name.String())
			buf.WriteString(" ")
			printer.Fprint(buf, fset, field.Type)
			if inField == argPos {
				buf.WriteString("⎬")
			}
		}
	}
	buf.WriteString(")")

	if fl := fx.Results; fl != nil {
		buf.WriteString(" ")
		hasNames := len(fl.List) != 0 && len(fl.List[0].Names) != 0
		if hasNames {
			buf.WriteString("(")
		}
		printFields(buf, fset, fl.List, true)
		if hasNames {
			buf.WriteString(")")
		}
	}

	return buf.String()
}

func (gc *GocodeCalltips) argPos(call *ast.CallExpr, tpos token.Pos) int {
	np := token.NoPos
	ne := token.NoPos
	for i, a := range call.Args {
		if np == token.NoPos {
			np = a.Pos()
		}
		ne = a.End()
		if np <= tpos && tpos <= ne {
			return i
		}
		np = a.End() + 1
	}
	return -1
}

func (gc *GocodeCalltips) candidate(mx *mg.Ctx, src []byte, pos int, funcName string) (candidate gocode.MargoCandidate, ok bool) {
	if pos < 0 || pos >= len(src) {
		return candidate, false
	}

	bctx := BuildContext(mx)
	candidates := gocode.Margo.Complete(gocode.MargoConfig{
		GOROOT:             bctx.GOROOT,
		GOPATHS:            PathList(bctx.GOPATH),
		UnimportedPackages: true,
	}, src, mx.View.Filename(), pos)

	for _, c := range candidates {
		if strings.HasPrefix(c.Type, "func(") && strings.EqualFold(funcName, c.Name) {
			return c, true
		}
	}
	return candidate, false
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
