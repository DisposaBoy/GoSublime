package goutil

import (
	"go/ast"
	"go/build"
	"go/token"
	"io"
	"margo.sh/mg"
	yotsuba "margo.sh/why_would_you_make_yotsuba_cry"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	// Langs is the list of all Go related langs
	Langs = []mg.Lang{mg.Go, mg.GoMod, mg.GoSum}
)

type SrcDirKey struct {
	GOROOT, GOPATH, SrcDir string
}

func MakeSrcDirKey(bctx *build.Context, srcDir string) SrcDirKey {
	return SrcDirKey{bctx.GOROOT, bctx.GOPATH, filepath.Clean(srcDir)}
}

func BuildContextWithoutCallbacks(mx *mg.Ctx) *build.Context {
	c := build.Default
	c.GOARCH = mx.Env.Get("GOARCH", c.GOARCH)
	c.GOOS = mx.Env.Get("GOOS", c.GOOS)
	// these must be passed by the client
	// if we leave them unset, there's a risk something will end up using os.Getenv(...)
	logUndefined := func(k string) string {
		v := mx.Env[k]
		if v == "" {
			v = k + "-is-not-defined"
			mx.Log.Println(v)
		}
		return v
	}
	c.GOROOT = logUndefined("GOROOT")
	c.GOPATH = logUndefined("GOPATH")
	return &c
}

func BuildContext(mx *mg.Ctx) *build.Context {
	c := BuildContextWithoutCallbacks(mx)
	c.ReadDir = mx.VFS.ReadDir
	c.IsDir = mx.VFS.IsDir
	c.HasSubdir = HasImportPath // rage against the ~~machine~~symlinks...
	c.OpenFile = func(p string) (io.ReadCloser, error) {
		if v := mx.View; v != nil && p == v.Path {
			return v.Open()
		}
		if v := mx.View; v != nil && v.Path != "" && filepath.Dir(v.Path) == filepath.Dir(p) {
			if b := mx.VFS.ReadBlob(p); b != nil {
				return b.OpenFile()
			}
		}
		if b := mx.VFS.PeekBlob(p); b != nil {
			return b.OpenFile()
		}
		return os.Open(p)
	}
	return c
}

// HasImportPath reports whether dir is lexically a subdirectory of root.
// If so, it sets importPath to a slash-separated path that
// can be joined to root to produce a path equivalent to dir.
//
// HasImportPath is an implementation of go/build.Context.HasSubdir
func HasImportPath(root, dir string) (importPath string, ok bool) {
	root = filepath.Clean(root)
	dir = filepath.Clean(dir)
	if !strings.HasPrefix(dir, root) || root == dir {
		return "", false
	}
	importPath = filepath.ToSlash(dir[len(root):])
	if !strings.HasPrefix(importPath, string(filepath.Separator)) {
		return "", false
	}
	return importPath[1:], true
}

func PathList(p string) []string {
	l := []string{}
	for _, s := range strings.Split(p, string(filepath.ListSeparator)) {
		s = filepath.Clean(s)
		if filepath.IsAbs(s) {
			l = append(l, s)
		}
	}
	return l
}

func NodeEnclosesPos(node ast.Node, pos token.Pos) bool {
	if yotsuba.IsNil(node) {
		return false
	}
	if np := node.Pos(); !np.IsValid() || pos <= np {
		return false
	}

	ne := node.End()
	var cmnt *ast.Comment
	switch x := node.(type) {
	case *ast.Comment:
		cmnt = x
	case *ast.CommentGroup:
		if l := x.List; len(l) != 0 {
			cmnt = l[len(l)-1]
		}
	}
	if cmnt != nil && strings.HasPrefix(cmnt.Text, "//") {
		// line comments' end don't include the newline
		ne++
	}
	return pos < ne || !ne.IsValid()
}

type PosEnd struct {
	P token.Pos
	E token.Pos
}

func (pe PosEnd) Pos() token.Pos {
	return pe.P
}

func (pe PosEnd) End() token.Pos {
	return pe.E
}

func IsLetter(ch rune) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_' || ch >= utf8.RuneSelf && unicode.IsLetter(ch)
}

func IsPkgDir(dir string) bool {
	if dir == "" || dir == "." {
		return false
	}

	f, err := os.Open(dir)
	if err != nil {
		return false
	}

	l, _ := f.Readdirnames(-1)
	for _, fn := range l {
		if strings.HasSuffix(fn, ".go") {
			return true
		}
	}
	return false
}

// DedentCompletion Dedents s then trims preceding and succeeding empty lines.
func DedentCompletion(s string) string {
	return strings.TrimFunc(Dedent(s), func(r rune) bool {
		return r == '\n' || r == '\r'
	})
}

// Dedent un-indents tab-indented lines is s.
func Dedent(s string) string {
	lines := strings.Split(s, "\n")
	trim := func(s string) int {
		i := 0
		for i < len(s) && s[i] == '\t' {
			i++
		}
		return i
	}
	max := 0
	for i, s := range lines {
		cut := trim(s)
		switch {
		case max == 0:
			max = cut
		case cut > max:
			cut = max
		}
		lines[i] = s[cut:]
	}
	return strings.Join(lines, "\n")

}
