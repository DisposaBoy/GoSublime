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
	if cfg, ok := mx.Config.(sublime.Config); ok {
		mx = mx.Copy(func(mx *mg.Ctx) {
			mx.State = mx.SetConfig(cfg.DisableGsFmt())
		})
	}

	if !mx.View.LangIs("go") {
		return mx.State
	}
	if _, ok := mx.Action.(mg.ViewFmt); !ok {
		return mx.State
	}

	fn := mx.View.Filename()
	src, err := mx.View.ReadAll()
	if err != nil {
		return mx.Errorf("failed to read %s: %s\n", fn, err)
	}

	src, err = fm.fmt(fn, src)
	if err != nil {
		return mx.Errorf("failed to fmt %s: %s\n", fn, err)
	}
	return mx.SetSrc(src)
}
