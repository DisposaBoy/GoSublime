package golang

import (
	"disposa.blue/margo/mg"
	"go/parser"
	"go/scanner"
	"go/token"
)

type SyntaxCheck struct {
	hash   string
	name   string
	issues mg.IssueSet
}

func (sc *SyntaxCheck) Reduce(mx *mg.Ctx) *mg.State {
	st := mx.State
	if !st.View.LangIs("go") {
		return st
	}
	switch mx.Action.(type) {
	case mg.ViewActivated, mg.ViewModified, mg.ViewSaved:
		st = sc.checkSyntax(st)
	}
	return st.AddIssues(sc.issues.AllInView(st.View)...)
}

func (sc *SyntaxCheck) checkSyntax(st *mg.State) *mg.State {
	// TODO: caching
	v := st.View
	src, err := v.ReadAll()
	if err != nil {
		return st.Errorf("cannot read: %s: %s", v.Filename(), err)
	}

	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, v.Filename(), src, parser.DeclarationErrors)
	el, _ := err.(scanner.ErrorList)
	sc.name = v.Name
	sc.hash = v.Hash
	sc.issues = make([]mg.Issue, len(el))
	for i, e := range el {
		sc.issues[i] = mg.Issue{
			Path:    v.Path,
			Name:    v.Name,
			Row:     e.Pos.Line - 1,
			Col:     e.Pos.Column - 1,
			Message: e.Msg,
			Tag:     mg.IssueError,
		}
	}
	return st
}
