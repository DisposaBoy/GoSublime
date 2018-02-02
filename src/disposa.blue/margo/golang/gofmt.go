package golang

import (
	"disposa.blue/margo/mg"
	"go/format"
)

func GoFmt(st mg.State, act mg.Action) mg.State {
	if !st.View.LangIs("go") {
		return st
	}
	if _, ok := act.(mg.ViewFmt); !ok {
		return st
	}

	fn := st.View.Filename()
	src, err := st.View.ReadAll()
	if err != nil {
		return st.Errorf("gofmt: failed to read %s: %s\n", fn, err)
	}

	src, err = format.Source(src)
	if err != nil {
		return st.Errorf("gofmt: failed to format %s: %s\n", fn, err)
	}

	return st.SetSrc(src)
}
