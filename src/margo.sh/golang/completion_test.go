package golang

import (
	"testing"
)

func TestCompletionScopeStringer(t *testing.T) {
	if cs := CursorScope(0); cs.String() == "" {
		t.Errorf("%#v doesn't have a String() value", cs)
	}
	for cs := cursorScopesStart; cs <= cursorScopesEnd; cs <<= 1 {
		if cs.String() == "" {
			t.Errorf("%#v doesn't have a String() value", cs)
		}
	}
}
