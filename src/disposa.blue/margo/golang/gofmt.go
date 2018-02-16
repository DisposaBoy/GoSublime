package golang

import (
	"disposa.blue/margo/mg"
	"disposa.blue/margo/sublime"
	"go/format"
)

var (
	GoFmt = &fmter{fmt: func(_ string, src []byte) ([]byte, error) {
		return format.Source(src)
	}}
)

type fmter struct {
	fmt func(fn string, src []byte) ([]byte, error)
}

func (fm *fmter) Reduce(mx *mg.Ctx) *mg.State {
	st := mx.State
	if cfg, ok := mx.Config.(sublime.Config); ok {
		st = st.SetConfig(cfg.DisableGsFmt())
	}

	if !mx.View.LangIs("go") {
		return st
	}
	if _, ok := mx.Action.(mg.ViewFmt); !ok {
		return st
	}

	fn := st.View.Filename()
	src, err := st.View.ReadAll()
	if err != nil {
		return st.Errorf("failed to read %s: %s\n", fn, err)
	}

	src, err = fm.fmt(fn, src)
	if err != nil {
		return st.Errorf("failed to fmt %s: %s\n", fn, err)
	}
	return st.SetSrc(src)
}
