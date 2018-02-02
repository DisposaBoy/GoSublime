package golang

import (
	"disposa.blue/margo/mg"
	"golang.org/x/tools/imports"
)

func GoImports(st mg.State, act mg.Action) mg.State {
	if !st.View.LangIs("go") {
		return st
	}
	if _, ok := act.(mg.ViewFmt); !ok {
		return st
	}

	fn := st.View.Filename()
	src, err := imports.Process(fn, st.View.Src, nil)
	if err != nil {
		return st.Errorf("goimports: failed to format %s: %s\n", fn, err)
	}
	return st.SetSrc(src)
}
