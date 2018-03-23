package golang

import (
	"disposa.blue/margo/mg"
	"go/ast"
	"go/build"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	CommonPatterns = append([]*regexp.Regexp{
		regexp.MustCompile(`^\s*(?P<path>.+?\.\w+):(?P<line>\d+:)(?P<column>\d+:?)?(?:(?P<tag>warning|error)[:])?(?P<message>.+?)(?: [(](?P<label>[-\w]+)[)])?$`),
		regexp.MustCompile(`(?P<message>can't load package: package .+: found packages .+ \((?P<path>.+?\.go)\).+)`),
	}, mg.CommonPatterns...)
)

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
	if node == nil {
		return false
	}
	// apparently node can be (*T)(nil)
	if reflect.ValueOf(node).IsNil() {
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

func (cn *CursorNode) ScanFile(af *ast.File) {
	pos := af.Package
	end := pos + token.Pos(len("package"))
	if af.Name != nil {
		end = pos + token.Pos(af.Name.End())
	}
	if cn.Pos >= pos && cn.Pos <= end {
		return
	}

	cn.Append(af)
	ast.Walk(cn, af)
	for _, cg := range af.Comments {
		for _, c := range cg.List {
			if NodeEnclosesPos(c, cn.Pos) {
				cn.Append(c)
			}
		}
	}
	cn.Node = cn.Nodes[len(cn.Nodes)-1]
	cn.Set(&cn.GenDecl)
	cn.Set(&cn.BlockStmt)
	cn.Set(&cn.BasicLit)
	cn.Set(&cn.CallExpr)
	cn.Set(&cn.Comment)
	cn.Set(&cn.ImportSpec)
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
	pf := ParseFile(kvs, "", src)
	cn := &CursorNode{
		AstFile:   pf.AstFile,
		TokenFile: pf.TokenFile,
		Pos:       token.Pos(pf.TokenFile.Base() + offset),
	}
	cn.ScanFile(cn.AstFile)
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
