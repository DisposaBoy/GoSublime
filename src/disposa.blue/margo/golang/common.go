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
	CommonPatterns = append(mg.CommonPatterns[:len(mg.CommonPatterns):len(mg.CommonPatterns)],
		regexp.MustCompile(`(?P<message>can't load package: package .+: found packages .+ \((?P<path>.+?\.go)\).+)`),
	)
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

	cn.Node = af
	ast.Walk(cn, af)
	for _, cg := range af.Comments {
		for _, c := range cg.List {
			if NodeEnclosesPos(c, cn.Pos) {
				cn.Comment = c
				cn.Node = c
			}
		}
	}
}

func (cn *CursorNode) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return cn
	}
	// if we're inside e.g. an unclosed string, the end will be invalid
	if !NodeEnclosesPos(node, cn.Pos) {
		return cn
	}

	cn.Node = node
	switch x := node.(type) {
	case *ast.GenDecl:
		cn.GenDecl = x
	case *ast.BlockStmt:
		cn.BlockStmt = x
	case *ast.BasicLit:
		cn.BasicLit = x
	case *ast.CallExpr:
		cn.CallExpr = x
	case *ast.Comment:
		// comments that appear here are only those that are attached to something else
		// we will traverse *all* comments after .Walk()
	case *ast.ImportSpec:
		cn.ImportSpec = x
	}
	return cn
}

func ParseCursorNode(sto *mg.Store, src []byte, offset int) *CursorNode {
	pf := ParseFile(sto, "", src)
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
