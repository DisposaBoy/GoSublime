package golang

import (
	"testing"
)

func TestParseNilVars(t *testing.T) {
	if NilAstFile == nil || NilTokenFile == nil {
		t.Errorf("impossibru")
	}
}
