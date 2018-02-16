package golang

import (
	"disposa.blue/margo/mg"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

func BuildContext(e mg.EnvMap) *build.Context {
	c := build.Default
	c.GOARCH = e.Get("GOARCH", c.GOARCH)
	c.GOOS = e.Get("GOOS", c.GOOS)
	// these must be passed by the client
	// if we leave them unset, there's a risk something will end up using os.Getenv(...)
	logUndefined := func(k string) string {
		v := e[k]
		if v == "" {
			v = k + "-is-not-defined"
			mg.Log.Println(v)
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
	Fset *token.FileSet

	Pos       token.Pos
	AstFile   *ast.File
	TokenFile *token.File

	ImportSpec *ast.ImportSpec
	Comment    *ast.Comment
	BlockStmt  *ast.BlockStmt
	CallExpr   *ast.CallExpr
	BasicLit   *ast.BasicLit
	Node       ast.Node
}

func (cn *CursorNode) ScanFile(af *ast.File) {
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

func ParseCursorNode(src []byte, offset int) *CursorNode {
	cn := &CursorNode{Fset: token.NewFileSet()}

	// TODO: caching
	cn.AstFile, _ = parser.ParseFile(cn.Fset, "_.go", src, parser.ParseComments)
	if cn.AstFile == nil {
		return cn
	}

	cn.TokenFile = cn.Fset.File(cn.AstFile.Pos())
	if cn.TokenFile == nil {
		return cn
	}

	cn.Pos = cn.TokenFile.Pos(offset)
	cn.ScanFile(cn.AstFile)
	return cn
}

func IsLetter(ch rune) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_' || ch >= utf8.RuneSelf && unicode.IsLetter(ch)
}
