package golang

import (
	"disposa.blue/margo/mg"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
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
	/**/
	//
	_ = ""
	_ = ast.Comment{}
	np := node.Pos()
	ne := node.End()
	if c, ok := node.(*ast.Comment); ok && !strings.HasSuffix(c.Text, "*/") {
		// line comments don't include the newline
		ne++
	}
	return (np.IsValid() && pos > np) && (pos < ne || !ne.IsValid())
}

type NearestNode struct {
	Fset *token.FileSet

	Pos       token.Pos
	AstFile   *ast.File
	TokenFile *token.File

	Comment      *ast.Comment
	CommentGroup *ast.CommentGroup
	BlockStmt    *ast.BlockStmt
	CallExpr     *ast.CallExpr
	BasicLit     *ast.BasicLit
	Node         ast.Node
}

func (nn *NearestNode) ScanFile(af *ast.File) {
	ast.Walk(nn, af)
	for _, cg := range af.Comments {
		if !NodeEnclosesPos(cg, nn.Pos) {
			continue
		}
		nn.CommentGroup = cg
		nn.Node = cg
		for _, c := range cg.List {
			if NodeEnclosesPos(c, nn.Pos) {
				nn.Comment = c
				nn.Node = c
				return
			}
		}
		return
	}
}

func (nn *NearestNode) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nn
	}
	// if we're inside e.g. an unclosed string, the end will be invalid
	if !NodeEnclosesPos(node, nn.Pos) {
		return nn
	}

	nn.Node = node
	switch x := node.(type) {
	case *ast.BlockStmt:
		nn.BlockStmt = x
	case *ast.BasicLit:
		nn.BasicLit = x
	case *ast.CallExpr:
		nn.CallExpr = x
	case *ast.Comment:
		nn.Comment = x
	}
	return nn
}

func ParseNearestNode(src []byte, offset int) *NearestNode {
	nn := &NearestNode{Fset: token.NewFileSet()}

	// TODO: caching
	nn.AstFile, _ = parser.ParseFile(nn.Fset, "_.go", src, parser.ParseComments)
	if nn.AstFile == nil {
		return nn
	}

	nn.TokenFile = nn.Fset.File(nn.AstFile.Pos())
	if nn.TokenFile == nil {
		return nn
	}

	nn.Pos = nn.TokenFile.Pos(offset)
	nn.ScanFile(nn.AstFile)
	return nn
}
