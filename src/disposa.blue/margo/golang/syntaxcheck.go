package golang

import (
	"disposa.blue/margo/mg"
	"go/scanner"
)

type SyntaxCheck struct{}

func (sc *SyntaxCheck) Reduce(mx *mg.Ctx) *mg.State {
	st := mx.State
	if !st.View.LangIs("go") {
		return st
	}

	v := st.View
	src, err := v.ReadAll()
	if err != nil {
		return st.Errorf("cannot read: %s: %s", v.Filename(), err)
	}

	type key struct{ hash string }
	k := key{mg.SrcHash(src)}
	if issues, ok := mx.Store.Get(k).(mg.IssueSet); ok {
		return st.AddIssues(issues...)
	}

	if !mx.ActionIs(mg.ViewActivated{}, mg.ViewModified{}, mg.ViewSaved{}, mg.QueryIssues{}) {
		return st
	}

	pf := ParseFile(mx.Store, v.Filename(), src)
	issues := sc.errsToIssues(v, pf.ErrorList)
	mx.Store.Put(k, issues)

	return st.AddIssues(issues...)
}

func (_ *SyntaxCheck) errsToIssues(v *mg.View, el scanner.ErrorList) mg.IssueSet {
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
