package main

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"runtime"
)

type jString string

func (s jString) String() string {
	return string(s)
}

func (s *jString) UnmarshalJSON(p []byte) error {
	if bytes.Equal(p, []byte("null")) {
		return nil
	}
	return json.Unmarshal(p, (*string)(s))
}

func errStr(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

func envSlice(envMap map[string]string) []string {
	env := []string{}
	for k, v := range envMap {
		env = append(env, k+"="+v)
	}
	if len(env) == 0 {
		env = os.Environ()
	}
	return env
}

func defaultEnv() map[string]string {
	return map[string]string{
		"GOROOT": runtime.GOROOT(),
		"GOARCH": runtime.GOARCH,
		"GOOS":   runtime.GOOS,
	}
}

func orString(a ...string) string {
	for _, s := range a {
		if s != "" {
			return s
		}
	}
	return ""
}

func parseAstFile(fn string, s string, mode parser.Mode) (fset *token.FileSet, af *ast.File, err error) {
	fset = token.NewFileSet()
	var src interface{}
	if s != "" {
		src = s
	}
	if fn == "" {
		fn = "<stdin>"
	}
	af, err = parser.ParseFile(fset, fn, src, mode)
	return
}

func newPrinter(tabIndent bool, tabWidth int) *printer.Config {
	mode := printer.UseSpaces
	if tabIndent {
		mode |= printer.TabIndent
	}
	return &printer.Config{
		Mode:     mode,
		Tabwidth: tabWidth,
	}
}

func printSrc(fset *token.FileSet, v interface{}, tabIndent bool, tabWidth int) (src string, err error) {
	p := newPrinter(tabIndent, tabWidth)
	buf := &bytes.Buffer{}
	if err = p.Fprint(buf, fset, v); err == nil {
		src = buf.String()
	}
	return
}
