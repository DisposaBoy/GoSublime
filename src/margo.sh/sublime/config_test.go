package sublime

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	if len(DefaultConfig.Values.EnabledForLangs) == 0 {
		t.Fatalf("DefaultConfig.Values.EnabledForLangs is empty")
	}
}
