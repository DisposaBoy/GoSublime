package golang

import (
	"bytes"
	"github.com/klauspost/asmfmt"
	"margo.sh/format"
	"margo.sh/mg"
)

// AsmFmt is a reducer that does code fmt'ing for `.s` files.
// It uses the package https://github.com/klauspost/asmfmt
type AsmFmt struct{ mg.ReducerType }

func (AsmFmt) Reduce(mx *mg.Ctx) *mg.State {
	if mx.View.Ext != ".s" {
		return mx.State
	}
	return format.FmtFunc{
		Langs:   nil, // we only want to check the extension
		Actions: commonFmtActions,
		Fmt: func(mx *mg.Ctx, src []byte) ([]byte, error) {
			return asmfmt.Format(bytes.NewReader(src))
		},
	}.Reduce(mx)
}
