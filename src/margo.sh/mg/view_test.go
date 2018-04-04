package mg

import (
	"testing"
)

func TestBytePos(t *testing.T) {
	src := []byte(`…x`)
	cases := []struct{ chr, byt int }{
		{0, 0},
		{1, 3},
		{2, 4},
	}
	for _, c := range cases {
		res := BytePos(src, c.chr)
		if res != c.byt {
			t.Errorf("BytePos(%d) failed: expected '%d', got '%d'", c.chr, c.byt, res)
		}
	}
}
