package golang

import (
	"go/ast"
	"margo.sh/mg"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

type TestCmds struct {
	mg.ReducerType

	// BenchArgs is a list of extra arguments to pass to `go test` for benchmarks
	// these are in addition to the usual `-test.run` and `-test.bench` args
	BenchArgs []string

	// TestArgs is a list of extra arguments to pass to `go test` for tests and examples
	// these are in addition to the usual `-test.run` arg
	TestArgs []string
}

func (tc *TestCmds) ReducerCond(mx *mg.Ctx) bool {
	return mx.LangIs(mg.Go)
}

func (tc *TestCmds) Reduce(mx *mg.Ctx) *mg.State {
	switch mx.Action.(type) {
	case mg.QueryTestCmds:
		return tc.queryTestCmds(mx)
	default:
		return mx.State
	}
}

func (tc *TestCmds) queryTestCmds(mx *mg.Ctx) *mg.State {
	dir := mx.View.Dir()
	bld := BuildContext(mx)
	pkg, err := bld.ImportDir(dir, 0)
	if pkg == nil {
		mx.Log.Println("TestCmds:", err)
		return mx.State
	}

	cmds := map[string]mg.UserCmdList{}
	for _, names := range [][]string{pkg.TestGoFiles, pkg.XTestGoFiles} {
		for _, nm := range names {
			tc.process(mx, cmds, filepath.Join(dir, nm))
		}
	}

	numCmds := len(cmds["Test"]) + len(cmds["Benchmark"]) + len(cmds["Exampe"])
	if numCmds == 0 {
		mx.Log.Println("TestCmds: no Test, Benchmarks or Examples found")
		return mx.State
	}

	cl := make(mg.UserCmdList, 0, 4+numCmds)
	cl = append(cl, mg.UserCmd{
		Name:  "go",
		Args:  tc.testArgs("."),
		Title: "Run all Tests and Examples",
	})
	for _, pfx := range []string{"Test", "Benchmark", "Example"} {
		if len(cmds[pfx]) == 0 {
			continue
		}

		cmd := mg.UserCmd{
			Name:  "go",
			Title: "Run all " + pfx + "s",
		}
		if pfx == "Benchmark" {
			cmd.Args = tc.benchArgs(".")
		} else {
			cmd.Args = tc.testArgs(pfx + ".+")
		}
		cl = append(cl, cmd)
	}
	for _, pfx := range []string{"Test", "Benchmark", "Example"} {
		l := cmds[pfx]
		sort.Sort(l)
		cl = append(cl, l...)
	}
	return mx.AddUserCmds(cl...)
}

func (tc *TestCmds) benchArgs(pat string) []string {
	return append([]string{"test", "-test.run=none", "-test.bench=" + pat}, tc.BenchArgs...)
}

func (tc *TestCmds) testArgs(pat string) []string {
	return append([]string{"test", "-test.run=" + pat}, tc.TestArgs...)
}

func (tc *TestCmds) process(mx *mg.Ctx, cmds map[string]mg.UserCmdList, fn string) {
	for _, d := range ParseFile(mx.Store, fn, nil).AstFile.Decls {
		fun, ok := d.(*ast.FuncDecl)
		if ok && fun.Name != nil {
			tc.processIdent(cmds, fun.Name)
		}
	}
}

func (tc *TestCmds) processIdent(cmds map[string]mg.UserCmdList, id *ast.Ident) {
	name, pfx, sfx, ok := tc.splitIdent(id)
	if !ok {
		return
	}

	cmd := mg.UserCmd{
		Name:  "go",
		Title: pfx + ": " + sfx,
	}
	if pfx == "Benchmark" {
		cmd.Args = tc.benchArgs("^" + name + "$")
	} else {
		cmd.Args = tc.testArgs("^" + name + "$")
	}
	cmds[pfx] = append(cmds[pfx], cmd)
}

func (tc *TestCmds) splitIdent(id *ast.Ident) (name, pfx, sfx string, ok bool) {
	nm := id.String()
	for _, pfx := range []string{"Test", "Benchmark", "Example"} {
		sfx := strings.TrimPrefix(nm, pfx)
		if sfx != nm {
			r, _ := utf8.DecodeRuneInString(sfx)
			return nm, pfx, sfx, unicode.IsUpper(r)
		}
	}
	return "", "", "", false
}
