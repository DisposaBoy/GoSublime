package golang

import (
	"margo.sh/golang/cursor"
	"margo.sh/mg"
)

const (
	AssignmentScope = cursor.AssignmentScope
	BlockScope      = cursor.BlockScope
	CommentScope    = cursor.CommentScope
	ConstScope      = cursor.ConstScope
	DeferScope      = cursor.DeferScope
	DocScope        = cursor.DocScope
	ExprScope       = cursor.ExprScope
	FileScope       = cursor.FileScope
	FuncDeclScope   = cursor.FuncDeclScope
	IdentScope      = cursor.IdentScope
	ImportPathScope = cursor.ImportPathScope
	ImportScope     = cursor.ImportScope
	PackageScope    = cursor.PackageScope
	ReturnScope     = cursor.ReturnScope
	SelectorScope   = cursor.SelectorScope
	StringScope     = cursor.StringScope
	TypeDeclScope   = cursor.TypeDeclScope
	VarScope        = cursor.VarScope
)

type CursorScope = cursor.CurScope
type CompletionScope = CursorScope

type DocNode = cursor.DocNode

type CompletionCtx = CursorCtx
type CursorCtx = cursor.CurCtx

// NewCompletionCtx is an alias of cursor.NewCurCtx
func NewCompletionCtx(mx *mg.Ctx, src []byte, pos int) *CompletionCtx {
	return cursor.NewCurCtx(mx, src, pos)
}

// NewViewCursorCtx is an alias of cursor.NewViewCurCtx
func NewViewCursorCtx(mx *mg.Ctx) *CursorCtx { return cursor.NewViewCurCtx(mx) }

// NewCursorCtx is an alias of cursor.NewCurCtx
func NewCursorCtx(mx *mg.Ctx, src []byte, pos int) *CursorCtx { return cursor.NewCurCtx(mx, src, pos) }
