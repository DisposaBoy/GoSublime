package golang

import (
	"bytes"
	"fmt"
	"go/format"
	"os/exec"

	"disposa.blue/margo/mg"
	"disposa.blue/margo/sublime"
)

var (
	GoFmt     = FmtFunc(goFmt)
	GoImports = FmtFunc(goImports)
)

type FmtFunc func(mx *mg.Ctx, src []byte) ([]byte, error)

func (ff FmtFunc) Reduce(mx *mg.Ctx) *mg.State {
	st := mx.State
	if cfg, ok := mx.Config.(sublime.Config); ok {
		st = st.SetConfig(cfg.DisableGsFmt())
	}

	if !mx.View.LangIs("go") {
		return st
	}
	if !mx.ActionIs(mg.ViewFmt{}, mg.ViewPreSave{}) {
		return st
	}

	fn := st.View.Filename()
	src, err := st.View.ReadAll()
	if err != nil {
		return st.Errorf("failed to read %s: %s\n", fn, err)
	}

	src, err = ff(mx, src)
	if err != nil {
		return st.Errorf("failed to fmt %s: %s\n", fn, err)
	}
	return st.SetSrc(src)
}

func goFmt(_ *mg.Ctx, src []byte) ([]byte, error) {
	return format.Source(src)
}

func goImports(mx *mg.Ctx, src []byte) ([]byte, error) {
	stdin := bytes.NewReader(src)
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	cmd := exec.Command("goimports", "-srcdir", mx.View.Filename())
	cmd.Env = mx.Env.Environ()
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	if stderr.Len() != 0 {
		return nil, fmt.Errorf("fmt completed successfully, but contains stderr output: %s", stderr.Bytes())
	}
	return stdout.Bytes(), nil
}
