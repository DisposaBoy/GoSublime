package golang

import (
	"go/scanner"
	"margo.sh/mg"
	"margo.sh/mgutil"
)

type SyntaxCheck struct {
	mg.ReducerType

	q *mgutil.ChanQ
}

func (sc *SyntaxCheck) RCond(mx *mg.Ctx) bool {
	return mx.LangIs(mg.Go)
}

func (sc *SyntaxCheck) RMount(mx *mg.Ctx) {
	sc.q = mgutil.NewChanQ(1)
	go sc.checker()
}

func (sc *SyntaxCheck) RUnmount(mx *mg.Ctx) {
	sc.q.Close()
}

func (sc *SyntaxCheck) Reduce(mx *mg.Ctx) *mg.State {
	switch mx.Action.(type) {
	case mg.ViewActivated, mg.ViewModified, mg.ViewSaved:
		sc.q.Put(mx)
	}
	return mx.State
}

func (sc *SyntaxCheck) checker() {
	for v := range sc.q.C() {
		sc.check(v.(*mg.Ctx))
	}
}

func (sc *SyntaxCheck) check(mx *mg.Ctx) {
	src, _ := mx.View.ReadAll()
	pf := ParseFile(mx.Store, mx.View.Filename(), src)
	type iKey struct{}
	mx.Store.Dispatch(mg.StoreIssues{
		IssueKey: mg.IssueKey{Key: iKey{}},
		Issues:   sc.errsToIssues(mx.View, pf.ErrorList),
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
			Tag:     mg.Error,
			Label:   "Go/SyntaxCheck",
		}
	}
	return issues
}
