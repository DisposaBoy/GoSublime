package mg

import (
	"fmt"
	"testing"
)

func TestIssueWriter(t *testing.T) {
	base := Issue{Label: "lbl", Tag: IssueWarning}
	w := &IssueWriter{
		Dir:      "/abc",
		Base:     base,
		Patterns: CommonPatterns,
	}
	fmt.Fprintln(w, "abc.go:555:666: hello world")
	fmt.Fprintln(w, "no match")
	fmt.Fprint(w, "abc.go:555:")
	fmt.Fprint(w, "666: hello\n")
	fmt.Fprintln(w, " world")
	fmt.Fprintln(w, "no match")
	w.Flush()

	expect := IssueSet{
		Issue{Path: "/abc/abc.go", Row: 555 - 1, Col: 666 - 1, Tag: base.Tag, Label: base.Label, Message: "hello world"},
		Issue{Path: "/abc/abc.go", Row: 555 - 1, Col: 666 - 1, Tag: base.Tag, Label: base.Label, Message: "hello\n world"},
	}
	issues := w.Issues()
	if !expect.Equal(issues) {
		t.Errorf("IssueWriter parsing failed. Expected %#v, got %#v", expect, issues)
	}
}
