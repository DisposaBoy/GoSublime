package golang

import (
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"go/types"
	"margo.sh/golang/gopkg"
	"margo.sh/golang/goutil"
	"margo.sh/kimporter"
	"margo.sh/mg"
	"margo.sh/mgpf"
	"margo.sh/mgutil"
	"path/filepath"
	"strings"
	"time"
)

type TypeCheck struct {
	mg.ReducerType

	q *mgutil.ChanQ
}

func (tc *TypeCheck) RCond(mx *mg.Ctx) bool {
	return mx.LangIs(mg.Go)
}

func (tc *TypeCheck) RMount(mx *mg.Ctx) {
	tc.q = mgutil.NewChanQ(1)
	go tc.checker()
}

func (tc *TypeCheck) RUnmount(mx *mg.Ctx) {
	tc.q.Close()
}

func (tc *TypeCheck) Reduce(mx *mg.Ctx) *mg.State {
	switch mx.Action.(type) {
	case mg.ViewActivated, mg.ViewModified, mg.ViewSaved:
		tc.q.Put(mx)
	}
	return mx.State
}

func (tc *TypeCheck) checker() {
	for v := range tc.q.C() {
		tc.check(v.(*mg.Ctx))
	}
}

func (tc *TypeCheck) check(mx *mg.Ctx) {
	defer mx.Begin(mg.Task{Title: "Go/TypeCheck"}).Done()

	v := mx.View
	start := time.Now()
	defer func() {
		if d := time.Since(start); d > 100*time.Millisecond {
			mx.Log.Dbg.Println("T/C", v.ShortFn(mx.Env), mgpf.D(d))
		}
	}()

	dir := v.Dir()
	importPath := "_"
	if p, err := gopkg.ImportDir(mx, dir); err == nil {
		importPath = p.ImportPath
	}
	kp := kimporter.New(mx, nil)
	fset, files, err := tc.parseFiles(mx)
	issues := tc.errToIssues(err)
	if err == nil && len(files) != 0 {
		cfg := types.Config{
			FakeImportC: true,
			Error: func(err error) {
				issues = append(issues, tc.errToIssues(err)...)
			},
			Importer: kp,
		}
		_, err = cfg.Check(importPath, fset, files, nil)
		issues = append(issues, tc.errToIssues(err)...)
	}
	if err != nil && len(issues) == 0 {
		issues = append(issues, mg.Issue{Message: err.Error()})
	}
	for i, isu := range issues {
		if isu.Path == "" {
			isu.Path = v.Path
			isu.Name = v.Name
		}
		isu.Label = "Go/TypeCheck"
		isu.Tag = mg.Error
		issues[i] = isu
	}

	type K struct{}
	mx.Store.Dispatch(mg.StoreIssues{
		IssueKey: mg.IssueKey{Key: K{}},
		Issues:   issues,
	})
}

func (tc *TypeCheck) parseFiles(mx *mg.Ctx) (*token.FileSet, []*ast.File, error) {
	v := mx.View
	fn := v.Filename()
	src, _ := v.ReadAll()
	if v.Path == "" {
		pf := goutil.ParseFile(mx, fn, src)
		files := []*ast.File{pf.AstFile}
		if files[0] == nil {
			files = nil
		}
		return pf.Fset, files, pf.Error
	}

	currNm := v.Basename()
	dir := v.Dir()
	bp, err := BuildContext(mx).ImportDir(dir, 0)
	if err != nil {
		return nil, nil, err
	}
	fset := token.NewFileSet()
	// TODO: caching...
	af, err := parser.ParseFile(fset, fn, src, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}
	pkgFiles := map[string][]*ast.File{}
	names := append(bp.GoFiles, bp.CgoFiles...)
	if strings.HasSuffix(fn, "_test.go") {
		names = append(names, bp.TestGoFiles...)
	}
	for _, nm := range names {
		if nm == currNm {
			continue
		}
		af, err := parser.ParseFile(fset, filepath.Join(dir, nm), nil, parser.ParseComments)
		if err != nil {
			return nil, nil, err
		}
		pkgFiles[af.Name.String()] = append(pkgFiles[af.Name.String()], af)
	}
	files := append(pkgFiles[af.Name.String()], af)
	return fset, files, nil
}

func (tc *TypeCheck) errToIssues(err error) mg.IssueSet {
	var issues mg.IssueSet
	switch e := err.(type) {
	case scanner.ErrorList:
		for _, err := range e {
			issues = append(issues, tc.errToIssues(err)...)
		}
	case scanner.Error:
		issues = append(issues, mg.Issue{
			Row:     e.Pos.Line - 1,
			Col:     e.Pos.Column - 1,
			Message: e.Msg,
		})
	case types.Error:
		p := e.Fset.Position(e.Pos)
		issues = append(issues, mg.Issue{
			Path:    p.Filename,
			Row:     p.Line - 1,
			Col:     p.Column - 1,
			Message: e.Msg,
		})
	}
	return issues
}
