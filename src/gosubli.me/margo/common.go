package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	sRuneError = eRune()
	osArch     = runtime.GOOS + "_" + runtime.GOARCH
)

type void struct{}

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

type jData []byte

func (d jData) MarshalJSON() ([]byte, error) {
	if len(d) == 0 {
		return []byte(`""`), nil
	}

	buf := bytes.NewBufferString(`"base64:`)
	enc := base64.NewEncoder(base64.StdEncoding, buf)

	for len(d) > 0 {
		r, n := utf8.DecodeRune(d)
		if r == utf8.RuneError {
			enc.Write(sRuneError)
		} else {
			enc.Write(d[:n])
		}
		d = d[n:]
	}

	enc.Close()
	buf.WriteByte('"')
	return buf.Bytes(), nil
}

func uid() string {
	return "mg#" + strconv.FormatUint(numbers.next(), 16)
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

func fiHasGoExt(fi os.FileInfo) bool {
	return strings.HasSuffix(fi.Name(), ".go")
}

func parsePkg(fset *token.FileSet, srcDir string, mode parser.Mode) (pkg *ast.Package, pkgs map[string]*ast.Package, err error) {
	if pkgs, err = parser.ParseDir(fset, srcDir, fiHasGoExt, mode); pkgs != nil {
		_, pkgName := filepath.Split(srcDir)
		// we aren't going to support package whose name don't match the directory unless it's main
		p, ok := pkgs[pkgName]
		if !ok {
			p, ok = pkgs["main"]
		}
		if ok {
			pkg, err = ast.NewPackage(fset, p.Files, nil, nil)
		}
	}
	return
}

func rootDirs(env map[string]string) []string {
	dirs := []string{}
	gopath := ""
	if len(env) == 0 {
		gopath = os.Getenv("GOPATH")
	} else {
		gopath = env["GOPATH"]
	}

	gorootBase := runtime.GOROOT()
	if len(env) > 0 && env["GOROOT"] != "" {
		gorootBase = env["GOROOT"]
	} else if fn := os.Getenv("GOROOT"); fn != "" {
		gorootBase = fn
	}
	goroot := filepath.Join(gorootBase, "src", "pkg")

	dirsSeen := map[string]bool{}
	for _, fn := range filepath.SplitList(gopath) {
		if dirsSeen[fn] {
			continue
		}
		dirsSeen[fn] = true

		// goroot may be a part of gopath and we don't want that
		if fn != "" && !strings.HasPrefix(fn, gorootBase) {
			fn := filepath.Join(fn, "src")
			if fi, err := os.Stat(fn); err == nil && fi.IsDir() {
				dirs = append(dirs, fn)
			}
		}
	}

	if fi, err := os.Stat(goroot); err == nil && fi.IsDir() {
		dirs = append(dirs, goroot)
	}

	return dirs
}

func findPkg(fset *token.FileSet, importPath string, dirs []string, mode parser.Mode) (pkg *ast.Package, pkgs map[string]*ast.Package, err error) {
	for _, dir := range dirs {
		srcDir := filepath.Join(dir, importPath)
		if pkg, pkgs, err = parsePkg(fset, srcDir, mode); pkg != nil {
			return
		}
	}
	return
}

func eRune() []byte {
	s := make([]byte, utf8.RuneLen(utf8.RuneError))
	n := utf8.EncodeRune(s, utf8.RuneError)
	s = s[:n]
	return s
}

func tempDir(env map[string]string, subDirs ...string) string {
	dir := ""

	if env != nil {
		for _, k := range []string{"TMP", "TMPDIR"} {
			dir = env[k]
			if dir != "" {
				break
			}
		}
	}

	if dir == "" {
		dir = os.TempDir()
	}

	args := append([]string{dir, "GoSublime-temp"}, subDirs...)
	dir = filepath.Join(args...)
	os.MkdirAll(dir, 0777)

	return dir
}

func post(r Response) {
	sendCh <- r
}

func dbg(format string, a ...interface{}) {
	postMessage("dbg: "+format, a...)
}

func postMessage(format string, a ...interface{}) {
	post(Response{
		Token: "margo.message",
		Data: M{
			"message": fmt.Sprintf(format, a...),
		},
	})
}

func fileImportPaths(af *ast.File) []string {
	l := []string{}

	if af != nil {
		for _, decl := range af.Decls {
			if gdecl, ok := decl.(*ast.GenDecl); ok {
				for _, spec := range gdecl.Specs {
					if ispec, ok := spec.(*ast.ImportSpec); ok && ispec.Path != nil {
						ipath := unquote(ispec.Path.Value)
						if ipath != "C" {
							l = append(l, ipath)
						}
					}
				}
			}
		}
	}

	return l
}

func pathList(p string) []string {
	l := []string{}
	for _, s := range strings.Split(p, string(filepath.ListSeparator)) {
		if s != "" {
			l = append(l, s)
		}
	}
	return l
}

func envRootList(env map[string]string) (string, []string) {
	if env == nil {
		return "", []string{}
	}
	return env["GOROOT"], pathList(env["GOPATH"])
}

func msDur(start time.Time) time.Duration {
	dur := time.Now().Sub(start)
	dur -= dur % time.Millisecond
	return dur
}

func bytePos(src string, charPos int) int {
	for i, _ := range src {
		if charPos <= 0 {
			return i
		}
		charPos--
	}
	return -1
}
