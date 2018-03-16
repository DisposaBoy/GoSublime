package golang

import (
	"disposa.blue/margo/mg"
	"go/ast"
	"go/token"
	"strings"
)

const (
	PackageScope CompletionScope = 1 << iota
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
)

type CompletionScope uint64

func (cs CompletionScope) Is(scope CompletionScope) bool {
	return cs&scope != 0
}

func (cs CompletionScope) Any(scopes ...CompletionScope) bool {
	for _, s := range scopes {
		if cs&s != 0 {
			return true
		}
	}
	return false
}

func (cs CompletionScope) All(scopes ...CompletionScope) bool {
	for _, s := range scopes {
		if cs&s == 0 {
			return false
		}
	}
	return true
}

type CompletionCtx struct {
	*mg.Ctx
	CursorNode *CursorNode
	AstFile    *ast.File
	Scope      CompletionScope
	PkgName    string
	IsTestFile bool
}

func NewCompletionCtx(mx *mg.Ctx, src []byte, pos int) *CompletionCtx {
	cn := ParseCursorNode(mx.Store, src, pos)
	af := cn.AstFile
	if af == nil {
		af = NilAstFile
	}
	cx := &CompletionCtx{
		Ctx:        mx,
		CursorNode: cn,
		AstFile:    af,
		PkgName:    af.Name.String(),
	}
	cx.IsTestFile = strings.HasSuffix(mx.View.Filename(), "_test.go") ||
		strings.HasSuffix(cx.PkgName, "_test")

	if cx.PkgName == "_" || cx.PkgName == "" {
		cx.Scope |= PackageScope
		return cx
	}

	switch cx.CursorNode.Node.(type) {
	case nil:
		cx.Scope |= PackageScope
	case *ast.File:
		cx.Scope |= FileScope
	case *ast.BlockStmt:
		cx.Scope |= BlockScope
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
	if cn.Comment != nil {
		cx.Scope |= CommentScope
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

func DedentCompletion(s string) string {
	s = strings.TrimLeft(s, "\n")
	sfx := strings.TrimLeft(s, " \t")
	pfx := s[:len(s)-len(sfx)]
	if pfx == "" {
		return s
	}
	s = strings.TrimSpace(s)
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimPrefix(ln, pfx)
	}
	return strings.Join(lines, "\n")
}
