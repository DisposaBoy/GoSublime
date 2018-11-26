package golang

import (
	"bytes"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"margo.sh/mg"
	"margo.sh/mgutil"
	yotsuba "margo.sh/why_would_you_make_yotsuba_cry"
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

type cursorNode struct {
	Pos       token.Pos
	AstFile   *ast.File
	TokenFile *token.File
	Doc       *DocNode

	GenDecl    *ast.GenDecl
	ImportSpec *ast.ImportSpec
	Comment    *ast.Comment
	BlockStmt  *ast.BlockStmt
	CallExpr   *ast.CallExpr
	BasicLit   *ast.BasicLit
	Nodes      []ast.Node
	Node       ast.Node
}

func (cn *cursorNode) append(n ast.Node) {
	// ignore bad nodes, they usually just make scope detection fail with no obvious benefit
	switch n.(type) {
	case *ast.BadDecl, *ast.BadExpr, *ast.BadStmt:
		return
	}

	for _, x := range cn.Nodes {
		if n == x {
			return
		}
	}
	cn.Nodes = append(cn.Nodes, n)
}

func (cn *cursorNode) Set(destPtr interface{}) bool {
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

func (cn *cursorNode) Each(f func(ast.Node)) {
	for i := len(cn.Nodes) - 1; i >= 0; i-- {
		f(cn.Nodes[i])
	}
}

func (cn *cursorNode) Some(f func(ast.Node) bool) bool {
	for i := len(cn.Nodes) - 1; i >= 0; i-- {
		if f(cn.Nodes[i]) {
			return true
		}
	}
	return false
}

func (cn *cursorNode) Contains(typ ast.Node) bool {
	t := reflect.TypeOf(typ)
	return cn.Some(func(n ast.Node) bool {
		return reflect.TypeOf(n) == t
	})
}

func (cn *cursorNode) init(kvs mg.KVStore, src []byte, offset int) {
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
	cn.AstFile = af
	cn.TokenFile = pf.TokenFile
	cn.Pos = token.Pos(pf.TokenFile.Base() + offset)

	cn.initDocNode(af)
	if astFileIsValid(af) && cn.Pos > af.Name.End() {
		cn.append(af)
		ast.Inspect(af, func(n ast.Node) bool {
			if NodeEnclosesPos(n, cn.Pos) {
				cn.append(n)
			}
			cn.initDocNode(n)
			return true
		})
	}

	for _, cg := range af.Comments {
		for _, c := range cg.List {
			if NodeEnclosesPos(c, cn.Pos) {
				cn.append(c)
			}
		}
	}

	if len(cn.Nodes) == 0 {
		return
	}
	cn.Node = cn.Nodes[len(cn.Nodes)-1]
	cn.Each(func(n ast.Node) {
		switch x := n.(type) {
		case *ast.GenDecl:
			cn.GenDecl = x
		case *ast.BlockStmt:
			cn.BlockStmt = x
		case *ast.BasicLit:
			cn.BasicLit = x
		case *ast.CallExpr:
			cn.CallExpr = x
		case *ast.Comment:
			cn.Comment = x
		case *ast.ImportSpec:
			cn.ImportSpec = x
		}
	})
}

func (cn *cursorNode) initDocNode(n ast.Node) {
	if cn.Doc != nil || yotsuba.IsNil(n) {
		return
	}

	setCg := func(cg *ast.CommentGroup) {
		if cn.Doc != nil || cg == nil || !NodeEnclosesPos(cg, cn.Pos) {
			return
		}
		cn.Doc = &DocNode{
			Node:         n,
			CommentGroup: *cg,
		}
	}

	switch x := n.(type) {
	case *ast.File:
		setCg(x.Doc)
	case *ast.Field:
		setCg(x.Doc)
	case *ast.GenDecl:
		setCg(x.Doc)
	case *ast.TypeSpec:
		setCg(x.Doc)
	case *ast.FuncDecl:
		setCg(x.Doc)
	case *ast.ValueSpec:
		setCg(x.Doc)
	case *ast.ImportSpec:
		setCg(x.Doc)
	}
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

func consumeLeft(src []byte, pos int, cond func(rune) bool) int {
	return mgutil.RepositionLeft(src, pos, cond)
}

func consumeRight(src []byte, pos int, cond func(rune) bool) int {
	return mgutil.RepositionRight(src, pos, cond)
}
