package golang

import (
	"testing"
)

func TestDedent(t *testing.T) {
	src := `
		type T struct {
			S string
		}
	`
	want := `type T struct {
	S string
}`
	got := Dedent(src)
	if want != got {
		t.Errorf("got `\n%s\n`\nwant `\n%s\n`", got, want)
	}
}
