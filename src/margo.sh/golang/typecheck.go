package golang

import (
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"go/types"
	"margo.sh/golang/goutil"
	"margo.sh/kimporter"
	"margo.sh/mg"
	"margo.sh/mgpf"
	"margo.sh/mgutil"
	"os"
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
	pf := mgpf.NewProfile("Go/TypeCheck")
	defer func() {
		if pf.Dur().Duration < 100*time.Millisecond {
			return
		}
		mx.Profile.Fprint(os.Stderr, &mgpf.PrintOpts{
			MinDuration: 10 * time.Millisecond,
		})
	}()
	mx = mx.Copy(func(mx *mg.Ctx) { mx.Profile = pf })
	v := mx.View

	src, _ := v.ReadAll()
	issues := []mg.Issue{}
	if v.Path == "" {
		pf := goutil.ParseFile(mx, v.Name, src)
		issues = append(issues, tc.errToIssues(v, pf.Error)...)
		if pf.Error == nil {
			tcfg := types.Config{
				IgnoreFuncBodies: true,
				FakeImportC:      true,
				Error: func(err error) {
					issues = append(issues, tc.errToIssues(v, err)...)
				},
				Importer: kimporter.New(mx, nil),
			}
			tcfg.Check("_", pf.Fset, []*ast.File{pf.AstFile}, nil)
		}
	} else {
		kp := kimporter.New(mx, &kimporter.Config{
			CheckFuncs:   true,
			CheckImports: true,
			Tests:        strings.HasSuffix(v.Filename(), "_test.go"),
			SrcMap:       map[string][]byte{v.Filename(): src},
		})
		_, err := kp.ImportFrom(".", v.Dir(), 0)
		issues = append(issues, tc.errToIssues(v, err)...)
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
	src, _ := v.ReadAll()
	if v.Path == "" {
		pf := goutil.ParseFile(mx, v.Name, src)
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
	fn := v.Filename()
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

func (tc *TypeCheck) errToIssues(v *mg.View, err error) mg.IssueSet {
	var issues mg.IssueSet
	switch e := err.(type) {
	case nil:
	case scanner.ErrorList:
		for _, err := range e {
			issues = append(issues, tc.errToIssues(v, err)...)
		}
	case mg.Issue:
		issues = append(issues, e)
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
	default:
		issues = append(issues, mg.Issue{
			Name:    v.Name,
			Message: err.Error(),
		})
	}
	return issues
}
