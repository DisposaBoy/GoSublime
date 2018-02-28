package golang

import (
	"fmt"
	"testing"
)

func TestVersion(t *testing.T) {
	s := fmt.Sprintf("go%d.%d", Version.Major, Version.Minor)
	if s != VersionTag {
		t.Errorf("Version is invalid. Expected %s, got %s", s, VersionTag)
	}
}
