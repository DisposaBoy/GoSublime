package mg

import (
	"fmt"
)

type IssueTag string

const (
	IssueError   = IssueTag("error")
	IssueWarning = IssueTag("warning")
)

type Issue struct {
	Path    string
	Name    string
	Row     int
	Col     int
	End     int
	Tag     IssueTag
	Label   string
	Message string
}

func (isu *Issue) InView(v *View) bool {
	if v.Path != "" && isu.Path != "" {
		return v.Path == isu.Path
	}
	return isu.Name == v.Name
}

func (isu *Issue) Valid() bool {
	return isu.Name != "" || isu.Path != ""
}

type IssueSet []Issue

func (s IssueSet) Add(l ...Issue) IssueSet {
	res := make(IssueSet, 0, len(s)+len(l))
	for _, lst := range []IssueSet{s, IssueSet(l)} {
		for _, p := range lst {
			if !res.Has(p) {
				res = append(res, p)
			}
		}
	}
	return res
}

func (s IssueSet) Has(p Issue) bool {
	for _, q := range s {
		if p == q {
			return true
		}
	}
	return false
}

func (is IssueSet) AllInView(v *View) IssueSet {
	issues := make(IssueSet, 0, len(is))
	for _, i := range is {
		if i.InView(v) {
			issues = append(issues, i)
		}
	}
	return issues
}

type issueSupport struct{}

func (_ issueSupport) Reduce(mx *Ctx) *State {
	status := make([]string, 0, 3)
	status = append(status, "placeholder")
	cnt := 0
	for _, isu := range mx.Issues {
		if !isu.InView(mx.View) {
			continue
		}
		cnt++
		if len(status) > 1 || isu.Message == "" || isu.Row != mx.View.Row {
			continue
		}
		if isu.Label != "" {
			status = append(status, isu.Label)
		}
		status = append(status, isu.Message)
	}
	switch cnt {
	case 0:
		status = nil
	case len(mx.Issues):
		status[0] = fmt.Sprintf("Issues (%d)", cnt)
	default:
		status[0] = fmt.Sprintf("Issues (%d/%d)", cnt, len(mx.Issues))
	}
	return mx.AddStatus(status...)
}
