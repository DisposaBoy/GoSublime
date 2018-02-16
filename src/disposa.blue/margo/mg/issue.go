package mg

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
	Message string
}

func (i *Issue) InView(v *View) bool {
	if v.Path != "" && i.Path != "" {
		return v.Path == i.Path
	}
	return i.Name == v.Name
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
	var issues IssueSet
	for _, i := range is {
		if i.InView(v) {
			issues = append(issues, i)
		}
	}
	return issues
}
