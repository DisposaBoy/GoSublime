package golang

import (
	"go/ast"
	"go/token"
	"margo.sh/mg"
	"sort"
	"strings"
)

const (
	cursorScopesStart CursorScope = 1 << iota
	PackageScope
	FileScope
	DeclScope
	BlockScope
	ImportScope
	ConstScope
	VarScope
	TypeScope
	CommentScope
	StringScope
	ImportPathScope
	cursorScopesEnd
)

var (
	cursorScopeNames = map[CursorScope]string{
		PackageScope:    "PackageScope",
		FileScope:       "FileScope",
		DeclScope:       "DeclScope",
		BlockScope:      "BlockScope",
		ImportScope:     "ImportScope",
		ConstScope:      "ConstScope",
		VarScope:        "VarScope",
		TypeScope:       "TypeScope",
		CommentScope:    "CommentScope",
		StringScope:     "StringScope",
		ImportPathScope: "ImportPathScope",
	}
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

func (cs CursorScope) Is(scope CursorScope) bool {
	return cs&scope != 0
}

func (cs CursorScope) Any(scopes ...CursorScope) bool {
	for _, s := range scopes {
		if cs&s != 0 {
			return true
		}
	}
	return false
}

func (cs CursorScope) All(scopes ...CursorScope) bool {
	for _, s := range scopes {
		if cs&s == 0 {
			return false
		}
	}
	return true
}

type CompletionCtx = CursorCtx
type CursorCtx struct {
	*mg.Ctx
	CursorNode *CursorNode
	AstFile    *ast.File
	Scope      CursorScope
	PkgName    string
	IsTestFile bool
}

func NewCompletionCtx(mx *mg.Ctx, src []byte, pos int) *CompletionCtx {
	return NewCursorCtx(mx, src, pos)
}

func NewViewCursorCtx(mx *mg.Ctx) *CursorCtx {
	src, pos := mx.View.SrcPos()
	return NewCursorCtx(mx, src, pos)
}

func NewCursorCtx(mx *mg.Ctx, src []byte, pos int) *CursorCtx {
	cn := ParseCursorNode(mx.Store, src, pos)
	af := cn.AstFile
	if af == nil {
		af = NilAstFile
	}
	cx := &CursorCtx{
		Ctx:        mx,
		CursorNode: cn,
		AstFile:    af,
		PkgName:    af.Name.String(),
	}
	cx.IsTestFile = strings.HasSuffix(mx.View.Filename(), "_test.go") ||
		strings.HasSuffix(cx.PkgName, "_test")

	if cn.Comment != nil {
		cx.Scope |= CommentScope
	}

	if cx.PkgName == NilPkgName || cx.PkgName == "" {
		cx.PkgName = NilPkgName
		cx.Scope |= PackageScope
		return cx
	}

	switch x := cx.CursorNode.Node.(type) {
	case nil:
		cx.Scope |= PackageScope
	case *ast.File:
		cx.Scope |= FileScope
	case *ast.BlockStmt:
		cx.Scope |= BlockScope
	case *ast.CaseClause:
		if NodeEnclosesPos(PosEnd{x.Colon, x.End()}, cn.Pos) {
			cx.Scope |= BlockScope
		}
	}

	if gd := cn.GenDecl; gd != nil {
		switch gd.Tok {
		case token.IMPORT:
			cx.Scope |= ImportScope
		case token.CONST:
			cx.Scope |= ConstScope
		case token.VAR:
			cx.Scope |= VarScope
		case token.TYPE:
			cx.Scope |= TypeScope
		}
	}

	if lit := cn.BasicLit; lit != nil && lit.Kind == token.STRING {
		if cn.ImportSpec != nil {
			cx.Scope |= ImportPathScope
		} else {
			cx.Scope |= StringScope
		}
	}

	return cx
}

func (cx *CursorCtx) funcName() (name string, isMethod bool) {
	var fd *ast.FuncDecl
	cn := cx.CursorNode
	if !cn.Set(&fd) {
		return "", false
	}
	if fd.Name == nil || !NodeEnclosesPos(fd.Name, cx.CursorNode.Pos) {
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
