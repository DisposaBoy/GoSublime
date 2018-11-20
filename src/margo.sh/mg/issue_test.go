package mg

import (
	"fmt"
	"testing"
)

func TestIssueWriter(t *testing.T) {
	base := Issue{Label: "lbl", Tag: Warning}
	w := &IssueOut{
		Dir:      "/abc",
		Base:     base,
		Patterns: CommonPatterns(),
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

func BenchmarkIssueSetAdd(b *testing.B) {
	// if we make a syntax error at the top of a large file
	// we can end up with thousands of errors
	large := make(IssueSet, 2000)
	for i, _ := range large {
		large[i] = Issue{Row: i, Col: i}
	}
	small := large[:100]

	run := func(b *testing.B, s, add IssueSet) {
		b.Helper()
		for i := 0; i < b.N; i++ {
			s.Add(add...)
		}
	}
	b.Run("empty, large", func(b *testing.B) { run(b, IssueSet{}, large) })
	b.Run("small, large", func(b *testing.B) { run(b, small, large) })
	b.Run("large, large", func(b *testing.B) { run(b, large, large) })
	b.Run("empty, small", func(b *testing.B) { run(b, IssueSet{}, small) })
	b.Run("small, small", func(b *testing.B) { run(b, small, small) })
	b.Run("large, small", func(b *testing.B) { run(b, large, small) })
}
