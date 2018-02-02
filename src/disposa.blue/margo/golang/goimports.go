package golang

import (
	"disposa.blue/margo/mg"
	"golang.org/x/tools/imports"
)

func GoImports(st mg.State, act mg.Action) mg.State {
	return fmt(st, act, func(v mg.View) ([]byte, error) {
		return imports.Process(st.View.Filename(), st.View.Src, nil)
	})
}
