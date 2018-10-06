package golang

import (
	"bytes"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"margo.sh/mg"
	"margo.sh/why_would_you_make_yotsuba_cry"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

func init() {
	mg.AddCommonPatterns(mg.Go,
		regexp.MustCompile(`^\s*(?P<path>.+?\.\w+):(?P<line>\d+:)(?P<column>\d+:?)?(?:(?P<tag>warning|error)[:])?(?P<message>.+?)(?: [(](?P<label>[-\w]+)[)])?$`),
		regexp.MustCompile(`(?P<message>can't load package: package .+: found packages .+ \((?P<path>.+?\.go)\).+)`),
	)
}

func BuildContext(mx *mg.Ctx) *build.Context {
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

func PathList(p string) []string {
	l := []string{}
	for _, s := range strings.Split(p, string(filepath.ListSeparator)) {
		if s != "" {
			l = append(l, s)
		}
	}
	return l
}

func NodeEnclosesPos(node ast.Node, pos token.Pos) bool {
	if why_would_you_make_yotsuba_cry.IsNil(node) {
		return false
	}
	if np := node.Pos(); !np.IsValid() || pos <= np {
		return false
	}
	ne := node.End()
	if c, ok := node.(*ast.Comment); ok && strings.HasPrefix(c.Text, "//") {
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

type CursorNode struct {
	Pos       token.Pos
	AstFile   *ast.File
	TokenFile *token.File

	GenDecl    *ast.GenDecl
	ImportSpec *ast.ImportSpec
	Comment    *ast.Comment
	BlockStmt  *ast.BlockStmt
	CallExpr   *ast.CallExpr
	BasicLit   *ast.BasicLit
	Nodes      []ast.Node
	Node       ast.Node
}

func (cn *CursorNode) Visit(node ast.Node) ast.Visitor {
	if NodeEnclosesPos(node, cn.Pos) {
		cn.Append(node)
	}
	return cn
}

func (cn *CursorNode) Append(n ast.Node) {
	for _, x := range cn.Nodes {
		if n == x {
			return
		}
	}
	cn.Nodes = append(cn.Nodes, n)
}

func (cn *CursorNode) Set(destPtr interface{}) bool {
	v := reflect.ValueOf(destPtr).Elem()
	if !v.CanSet() {
		return false
	}
	for i := len(cn.Nodes) - 1; i >= 0; i-- {
		x := reflect.ValueOf(cn.Nodes[i])
		if x.Type() == v.Type() {
			v.Set(x)
			return true
		}
	}
	return false
}

func ParseCursorNode(kvs mg.KVStore, src []byte, offset int) *CursorNode {
	astFileIsValid := func(af *ast.File) bool {
		return af.Package.IsValid() &&
			af.Name != nil &&
			af.Name.End().IsValid() &&
			af.Name.Name != ""
	}
	srcHasComments := func() bool {
		return bytes.Contains(src, []byte("//")) || bytes.Contains(src, []byte("/*"))
	}

	pf := ParseFile(kvs, "", src)
	if !astFileIsValid(pf.AstFile) && srcHasComments() {
		// we don't want any declaration errors esp. about the package name `_`
		// we don't parse with this mode by default to increase the chance of caching
		s := append(src[:len(src):len(src)], NilPkgSrc...)
		pf = ParseFileWithMode(kvs, "", s, parser.ParseComments)
	}

	af := pf.AstFile
	cn := &CursorNode{
		AstFile:   af,
		TokenFile: pf.TokenFile,
		Pos:       token.Pos(pf.TokenFile.Base() + offset),
	}

	if astFileIsValid(af) && cn.Pos > af.Name.End() {
		cn.Append(af)
		ast.Walk(cn, af)
	}

	for _, cg := range af.Comments {
		for _, c := range cg.List {
			if NodeEnclosesPos(c, cn.Pos) {
				cn.Append(c)
			}
		}
	}

	if len(cn.Nodes) != 0 {
		cn.Node = cn.Nodes[len(cn.Nodes)-1]
		cn.Set(&cn.GenDecl)
		cn.Set(&cn.BlockStmt)
		cn.Set(&cn.BasicLit)
		cn.Set(&cn.CallExpr)
		cn.Set(&cn.Comment)
		cn.Set(&cn.ImportSpec)
	}

	return cn
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

func DedentCompletion(s string) string { return Dedent(s) }

func Dedent(s string) string {
	s = strings.TrimLeft(s, "\n")
	sfx := strings.TrimLeft(s, " \t")
	pfx := s[:len(s)-len(sfx)]
	if pfx == "" {
		return s
	}
	s = strings.TrimSpace(s)
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimPrefix(ln, pfx)
	}
	return strings.Join(lines, "\n")
}
