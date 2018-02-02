package golang

import (
	"disposa.blue/margo/mg"
	"disposa.blue/margo/sublime"
	"go/format"
)

func fmt(st mg.State, act mg.Action, fmt func(mg.View) ([]byte, error)) mg.State {
	if cfg, ok := st.Config.(sublime.Config); ok {
		st = st.SetConfig(cfg.DisableGsFmt())
	}

	if !st.View.LangIs("go") {
		return st
	}
	if _, ok := act.(mg.ViewFmt); !ok {
		return st
	}

	src, err := fmt(st.View)
	if err != nil {
		return st.Errorf("failed to fmt %s: %s\n", st.View.Filename(), err)
	}
	return st.SetSrc(src)
}

func GoFmt(st mg.State, act mg.Action) mg.State {
	return fmt(st, act, func(v mg.View) ([]byte, error) {
		src, err := st.View.ReadAll()
		if err != nil {
			return nil, err
		}
		return format.Source(src)
	})
}
