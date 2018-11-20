package golang

import (
	"bytes"
	"go/ast"
	"go/token"
	"margo.sh/mg"
	"margo.sh/mgutil"
	"sort"
	"strings"
)

const (
	cursorScopesStart CursorScope = 1 << iota
	AssignmentScope
	BlockScope
	CommentScope
	ConstScope
	DeferScope
	DocScope
	ExprScope
	FileScope
	FuncDeclScope
	IdentScope
	ImportPathScope
	ImportScope
	PackageScope
	ReturnScope
	SelectorScope
	StringScope
	TypeDeclScope
	VarScope
	cursorScopesEnd
)

var (
	cursorScopeNames = map[CursorScope]string{
		AssignmentScope: "AssignmentScope",
		BlockScope:      "BlockScope",
		CommentScope:    "CommentScope",
		ConstScope:      "ConstScope",
		DeferScope:      "DeferScope",
		DocScope:        "DocScope",
		ExprScope:       "ExprScope",
		FileScope:       "FileScope",
		FuncDeclScope:   "FuncDeclScope",
		IdentScope:      "IdentScope",
		ImportPathScope: "ImportPathScope",
		ImportScope:     "ImportScope",
		PackageScope:    "PackageScope",
		ReturnScope:     "ReturnScope",
		SelectorScope:   "SelectorScope",
		StringScope:     "StringScope",
		TypeDeclScope:   "TypeDeclScope",
		VarScope:        "VarScope",
	}

	_ ast.Node = (*DocNode)(nil)
)

type CursorScope uint64
type CompletionScope = CursorScope

func (cs CursorScope) String() string {
	if cs <= cursorScopesStart || cs >= cursorScopesEnd {
		return "UnknownCursorScope"
	}
	l := []string{}
	for scope, name := range cursorScopeNames {
		if cs.Is(scope) {
			l = append(l, name)
		}
	}
	sort.Strings(l)
	return strings.Join(l, "|")
}

func (cs CursorScope) Is(scopes ...CursorScope) bool {
	for _, s := range scopes {
		if s&cs != 0 {
			return true
		}
	}
	return false
}

type DocNode struct {
	Node ast.Node
	ast.CommentGroup
}

type CompletionCtx = CursorCtx
type CursorCtx struct {
	cursorNode
	Ctx        *mg.Ctx
	View       *mg.View
	Scope      CursorScope
	PkgName    string
	IsTestFile bool
	Line       []byte
}

func NewCompletionCtx(mx *mg.Ctx, src []byte, pos int) *CompletionCtx {
	return NewCursorCtx(mx, src, pos)
}

func NewViewCursorCtx(mx *mg.Ctx) *CursorCtx {
	src, pos := mx.View.SrcPos()
	return NewCursorCtx(mx, src, pos)
}

func NewCursorCtx(mx *mg.Ctx, src []byte, pos int) *CursorCtx {
	pos = mgutil.ClampPos(src, pos)

	// if we're at the end of the line, move the cursor onto the last thing on the line
	space := func(r rune) bool { return r == ' ' || r == '\t' }
	if i := mgutil.RepositionRight(src, pos, space); i < len(src) && src[i] == '\n' {
		pos = mgutil.RepositionLeft(src, pos, space)
		if j := pos - 1; j >= 0 && src[j] != '\n' && src[j] != '}' {
			pos = j
		}
	}

	ll := mgutil.RepositionLeft(src, pos, func(r rune) bool { return r != '\n' })
	lr := mgutil.RepositionRight(src, pos, func(r rune) bool { return r != '\n' })
	cx := &CursorCtx{
		Ctx:  mx,
		View: mx.View,
		Line: bytes.TrimSpace(src[ll:lr]),
	}
	cx.init(mx.Store, src, pos)

	af := cx.AstFile
	if af == nil {
		af = NilAstFile
	}
	cx.PkgName = af.Name.String()

	cx.IsTestFile = strings.HasSuffix(mx.View.Filename(), "_test.go") ||
		strings.HasSuffix(cx.PkgName, "_test")

	if cx.Comment != nil {
		cx.Scope |= CommentScope
	}
	if cx.Doc != nil {
		cx.Scope |= DocScope
		cx.Scope |= CommentScope
	}

	if cx.PkgName == NilPkgName || cx.PkgName == "" {
		cx.PkgName = NilPkgName
		cx.Scope |= PackageScope
		return cx
	}

	switch x := cx.Node.(type) {
	case nil:
		cx.Scope |= PackageScope
	case *ast.File:
		cx.Scope |= FileScope
	case *ast.BlockStmt:
		cx.Scope |= BlockScope
	case *ast.CaseClause:
		if NodeEnclosesPos(PosEnd{x.Colon, x.End()}, cx.Pos) {
			cx.Scope |= BlockScope
		}
	case *ast.Ident:
		cx.Scope |= IdentScope
	}

	cx.Each(func(n ast.Node) {
		switch n.(type) {
		case *ast.AssignStmt:
			cx.Scope |= AssignmentScope
		case *ast.SelectorExpr:
			cx.Scope |= SelectorScope
		case *ast.ReturnStmt:
			cx.Scope |= ReturnScope
		case *ast.DeferStmt:
			cx.Scope |= DeferScope
		}
	})

	if gd := cx.GenDecl; gd != nil {
		switch gd.Tok {
		case token.IMPORT:
			cx.Scope |= ImportScope
		case token.CONST:
			cx.Scope |= ConstScope
		case token.VAR:
			cx.Scope |= VarScope
		}
	}

	if lit := cx.BasicLit; lit != nil && lit.Kind == token.STRING {
		cx.Scope |= StringScope
		if cx.ImportSpec != nil {
			cx.Scope |= ImportPathScope
		}
	}

	// we want to allow `kw`, `kw name`, `kw (\n|\n)`
	punct := func(r rune) bool { return r != ' ' && r != '\t' && !IsLetter(r) }
	if cx.Scope == 0 && bytes.IndexFunc(cx.Line, punct) < 0 {
		switch x := cx.Node.(type) {
		case *ast.FuncType:
			cx.Scope |= FuncDeclScope
		case *ast.GenDecl:
			if x.Tok == token.TYPE {
				cx.Scope |= TypeDeclScope
			}
		}
	}

	exprOk := cx.Scope.Is(
		AssignmentScope,
		BlockScope,
		ConstScope,
		DeferScope,
		ReturnScope,
		VarScope,
	) && !cx.Scope.Is(
		SelectorScope,
		StringScope,
		CommentScope,
	)
	if asn := (*ast.AssignStmt)(nil); exprOk && cx.Set(&asn) {
		exprOk = pos >= cx.TokenFile.Offset(asn.TokPos)+len(asn.Tok.String())
	}
	if exprOk {
		cx.Scope |= ExprScope
	}

	return cx
}

func (cx *CursorCtx) funcName() (name string, isMethod bool) {
	var fd *ast.FuncDecl
	if !cx.Set(&fd) {
		return "", false
	}
	if fd.Name == nil || !NodeEnclosesPos(fd.Name, cx.Pos) {
		return "", false
	}
	return fd.Name.Name, fd.Recv != nil
}

// FuncName returns the name of function iff the cursor is on a func declariton's name
func (cx *CursorCtx) FuncName() string {
	if nm, isMeth := cx.funcName(); !isMeth {
		return nm
	}
	return ""
}

// FuncName returns the name of function iff the cursor is on a method declariton's name
func (cx *CursorCtx) MethodName() string {
	if nm, isMeth := cx.funcName(); isMeth {
		return nm
	}
	return ""
}
