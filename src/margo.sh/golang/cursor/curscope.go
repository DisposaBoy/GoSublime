package cursor

import (
	"sort"
	"strings"
)

const (
	curScopesStart CurScope = 1 << iota
	AssignmentScope
	BlockScope
	CommentScope
	ConstScope
	DeferScope
	DocScope
	ExprScope
	FileScope
	FuncDeclScope
	IdentScope
	ImportPathScope
	ImportScope
	PackageScope
	ReturnScope
	SelectorScope
	StringScope
	TypeDeclScope
	VarScope
	curScopesEnd
)

var (
	scopeNames = map[CurScope]string{
		AssignmentScope: "AssignmentScope",
		BlockScope:      "BlockScope",
		CommentScope:    "CommentScope",
		ConstScope:      "ConstScope",
		DeferScope:      "DeferScope",
		DocScope:        "DocScope",
		ExprScope:       "ExprScope",
		FileScope:       "FileScope",
		FuncDeclScope:   "FuncDeclScope",
		IdentScope:      "IdentScope",
		ImportPathScope: "ImportPathScope",
		ImportScope:     "ImportScope",
		PackageScope:    "PackageScope",
		ReturnScope:     "ReturnScope",
		SelectorScope:   "SelectorScope",
		StringScope:     "StringScope",
		TypeDeclScope:   "TypeDeclScope",
		VarScope:        "VarScope",
	}
)

type CurScope uint64

func (cs CurScope) String() string {
	if cs <= curScopesStart || cs >= curScopesEnd {
		return "UnknownCursorScope"
	}
	l := []string{}
	for scope, name := range scopeNames {
		if cs.Is(scope) {
			l = append(l, name)
		}
	}
	sort.Strings(l)
	return strings.Join(l, "|")
}

func (cs CurScope) Is(scopes ...CurScope) bool {
	for _, s := range scopes {
		if s&cs != 0 {
			return true
		}
	}
	return false
}
