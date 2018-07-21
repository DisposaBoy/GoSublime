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
