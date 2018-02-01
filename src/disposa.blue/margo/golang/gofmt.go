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

	src, err := st.View.ReadAll()
	if err != nil {
		mg.Log.Printf("gofmt: failed to read %s: %s\n", st.View.Path, err)
		return st
	}

	src, err = format.Source(src)
	if err != nil {
		mg.Log.Printf("gofmt: failed to format %s: %s\n", st.View.Path, err)
		return st
	}

	return st.SetSrc(src)
}
