package format

import (
	"bytes"
	"fmt"
	"margo.sh/mg"
	"os/exec"
)

// FmtFunc is a reducer for generic fmt functions
//
// it takes care of reading the view src and properly reporting any errors to the editor
type FmtFunc struct {
	// Fmt receives a copy of the view src and returns the fmt'ed src.
	//
	// Fmt should ideally fail in the face of any uncertainty
	// e.g. if running a command to do the formatting and it prints anything to stderr;
	// it should return an error because commands do not reliably return an error status.
	Fmt func(mx *mg.Ctx, src []byte) ([]byte, error)

	// Langs is the list of languages in which the reducer should run
	Langs []string

	// Actions is a list of additional actions on which the reducer is allowed to run.
	// The reducer always runs on the ViewFmt action, even if this list is empty.
	Actions []mg.Action
}

// Reduce implements the FmtFunc reducer.
func (ff FmtFunc) Reduce(mx *mg.Ctx) *mg.State {
	if !mx.LangIs(ff.Langs...) {
		return mx.State
	}
	if !mx.ActionIs(mg.ViewFmt{}) && !mx.ActionIs(ff.Actions...) {
		return mx.State
	}

	fn := mx.View.Filename()
	src, err := mx.View.ReadAll()
	if err != nil {
		return mx.Errorf("failed to read %s: %s\n", fn, err)
	}
	if len(src) == 0 {
		return mx.State
	}

	src, err = ff.Fmt(mx, src)
	if err != nil {
		return mx.Errorf("failed to fmt %s: %s\n", fn, err)
	}
	return mx.SetSrc(src)
}

// FmtCmd is wrapper around FmtFunc for generic fmt commands.
//
// The view src is passed to the command's stdin.
// It takes care of handling command failure e.g. output on stderr or no output on stdout.
type FmtCmd struct {
	// Name is the command name or path
	Name string

	// Args is a list of args to pass to the command.
	Args []string

	// Env is a map of additional env vars to pass to the command.
	Env mg.EnvMap

	// Langs is the list of languages in which the reducer should run
	Langs []string

	// Actions is a list of additional actions on which the reducer is allowed to run.
	// The reducer always runs on the ViewFmt action, even if this list is empty.
	Actions []mg.Action
}

// Reduce implements the FmtCmd reducer.
func (fc FmtCmd) Reduce(mx *mg.Ctx) *mg.State {
	return FmtFunc{Fmt: fc.fmt, Langs: fc.Langs, Actions: fc.Actions}.Reduce(mx)
}

func (fc FmtCmd) fmt(mx *mg.Ctx, src []byte) ([]byte, error) {
	stdin := bytes.NewReader(src)
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	cmd := exec.Command(fc.Name, fc.Args...)
	cmd.Env = mx.Env.Merge(fc.Env).Environ()
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	if stderr.Len() != 0 {
		return nil, fmt.Errorf("fmt completed successfully, but has output on stderr: %s", stderr.Bytes())
	}
	if stdout.Len() == 0 {
		return nil, fmt.Errorf("fmt completed successfully, but has no output on stdout")
	}
	return stdout.Bytes(), nil
}
