package why_would_you_make_yotsuba_cry

import (
	"errors"
	"testing"
)

func TestIsNil(t *testing.T) {
	num := 0
	cases := []struct {
		v interface{}
		b bool
	}{
		{nil, true},
		{0, false},
		{0.0, false},
		{"", false},
		{struct{}{}, false},
		{&struct{}{}, false},

		{(chan struct{})(nil), true},
		{(func())(nil), true},
		{(map[struct{}]struct{})(nil), true},
		{(*int)(nil), true},
		{(error)(nil), true},
		{[]byte(nil), true},

		{make(chan struct{}), false},
		{func() {}, false},
		{map[struct{}]struct{}{}, false},
		{&num, false},
		{errors.New(""), false},
		{[]byte{}, false},
	}
	for _, c := range cases {
		if res := IsNil(c.v); res != c.b {
			t.Errorf("IsNil(%#v) should return %v, not %v", c.v, c.b, res)
		}
	}
}
