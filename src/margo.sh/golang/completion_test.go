package golang

import (
	"testing"
)

func TestDedentCompletion(t *testing.T) {
	src := `
		type T struct {
			S string
		}
	`
	want := `type T struct {
	S string
}`
	got := DedentCompletion(src)
	if want != got {
		t.Errorf("got `\n%s\n`\nwant `\n%s\n`", got, want)
	}
}

func TestCompletionScopeStringer(t *testing.T) {
	if cs := CompletionScope(0); cs.String() == "" {
		t.Errorf("%#v doesn't have a String() value", cs)
	}
	for cs := completionScopesStart; cs <= completionScopesEnd; cs <<= 1 {
		if cs.String() == "" {
			t.Errorf("%#v doesn't have a String() value", cs)
		}
	}
}
