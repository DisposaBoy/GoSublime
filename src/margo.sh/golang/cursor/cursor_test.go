package cursor

import (
	"testing"
)

func TestCurScopeStringer(t *testing.T) {
	if cs := CurScope(0); cs.String() == "" {
		t.Errorf("%#v doesn't have a String() value", cs)
	}
	for cs := curScopesStart; cs <= curScopesEnd; cs <<= 1 {
		if cs.String() == "" {
			t.Errorf("%#v doesn't have a String() value", cs)
		}
	}
}
