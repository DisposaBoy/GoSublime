package golang

import (
	"go/scanner"
	"margo.sh/mg"
)

type SyntaxCheck struct{ mg.ReducerType }

func (sc *SyntaxCheck) Reduce(mx *mg.Ctx) *mg.State {
	if mx.LangIs("go") && mx.ActionIs(mg.ViewActivated{}, mg.ViewModified{}, mg.ViewSaved{}) {
		go sc.check(mx)
	}
	return mx.State
}

func (sc *SyntaxCheck) check(mx *mg.Ctx) {
	src, _ := mx.View.ReadAll()
	pf := ParseFile(mx.Store, mx.View.Filename(), src)
	type Key struct{}
	mx.Store.Dispatch(mg.StoreIssues{
		Key: mg.IssueKey{
			Key:  Key{},
			Name: mx.View.Name,
		},
		Issues: sc.errsToIssues(mx.View, pf.ErrorList),
	})
}

func (sc *SyntaxCheck) errsToIssues(v *mg.View, el scanner.ErrorList) mg.IssueSet {
	issues := make(mg.IssueSet, len(el))
	for i, e := range el {
		issues[i] = mg.Issue{
			Path:    v.Path,
			Name:    v.Name,
			Row:     e.Pos.Line - 1,
			Col:     e.Pos.Column - 1,
			Message: e.Msg,
			Tag:     mg.IssueError,
			Label:   "Go/SyntaxCheck",
		}
	}
	return issues
}
