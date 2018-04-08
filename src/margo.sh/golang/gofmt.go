package golang

import (
	"go/format"
	mgformat "margo.sh/format"
	"margo.sh/mg"
	"margo.sh/sublime"
)

var (
	GoFmt     mg.Reducer = mg.Reduce(goFmt)
	GoImports mg.Reducer = mg.Reduce(goImports)
)

func disableGsFmt(st *mg.State) *mg.State {
	if cfg, ok := st.Config.(sublime.Config); ok {
		return st.SetConfig(cfg.DisableGsFmt())
	}
	return st
}

type FmtFunc func(mx *mg.Ctx, src []byte) ([]byte, error)

func (ff FmtFunc) Reduce(mx *mg.Ctx) *mg.State {
	return disableGsFmt(mgformat.FmtFunc{
		Fmt:     ff,
		Langs:   []string{"go"},
		Actions: []mg.Action{mg.ViewPreSave{}},
	}.Reduce(mx))
}

func goFmt(mx *mg.Ctx) *mg.State {
	return FmtFunc(func(_ *mg.Ctx, src []byte) ([]byte, error) {
		return format.Source(src)
	}).Reduce(mx)
}

func goImports(mx *mg.Ctx) *mg.State {
	return disableGsFmt(mgformat.FmtCmd{
		Name:    "goimports",
		Args:    []string{"-srcdir", mx.View.Filename()},
		Langs:   []string{"go"},
		Actions: []mg.Action{mg.ViewPreSave{}},
	}.Reduce(mx))
}
