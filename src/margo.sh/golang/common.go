package golang

import (
	"go/ast"
	"go/build"
	"go/token"
	"margo.sh/golang/goutil"
	"margo.sh/mg"
	"regexp"
)

func init() {
	mg.AddCommonPatterns(mg.Go,
		regexp.MustCompile(`^\s*(?P<path>.+?\.\w+):(?P<line>\d+:)(?P<column>\d+:?)?(?:(?P<tag>warning|error)[:])?(?P<message>.+?)(?: [(](?P<label>[-\w]+)[)])?$`),
		regexp.MustCompile(`(?P<message>can't load package: package .+: found packages .+ \((?P<path>.+?\.go)\).+)`),
	)
}

// BuildContext is an alias of goutil.BuildContext
func BuildContext(mx *mg.Ctx) *build.Context { return goutil.BuildContext(mx) }

// PathList is an alias of goutil.PathList
func PathList(p string) []string { return goutil.PathList(p) }

// NodeEnclosesPos is an alias of goutil.NodeEnclosesPos
func NodeEnclosesPos(node ast.Node, pos token.Pos) bool { return goutil.NodeEnclosesPos(node, pos) }

// PosEnd is an alias of goutil.PosEnd
type PosEnd = goutil.PosEnd

// IsLetter is an alias of goutil.IsLetter
func IsLetter(ch rune) bool { return goutil.IsLetter(ch) }

// IsPkgDir is an alias of goutil.IsPkgDir
func IsPkgDir(dir string) bool { return goutil.IsPkgDir(dir) }

// DedentCompletion is an alias of goutil.DedentCompletion
func DedentCompletion(s string) string { return goutil.DedentCompletion(s) }

// Dedent is an alias of goutil.Dedent
func Dedent(s string) string { return goutil.Dedent(s) }
