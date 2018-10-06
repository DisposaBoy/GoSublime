package mgutil

import (
	"testing"
)

func TestClamp(t *testing.T) {
	type Case struct{ lo, hi, n, res int }

	test := func(c Case) {
		t.Helper()

		if got := Clamp(c.lo, c.hi, c.n); got != c.res {
			t.Errorf("Clamp(%d,%d, %d) should be %d, not %d", c.lo, c.hi, c.n, c.res, got)
		}
	}

	test(Case{-1, -1, 0, -1})
	test(Case{-1, 0, 0, 0})
	test(Case{-1, 1, 0, 0})
	test(Case{-1, 1, 1, 1})
	test(Case{-1, 1, 2, 1})
}

func TestClampPos(t *testing.T) {
	type Case struct{ len, pos, res int }

	test := func(c Case) {
		t.Helper()

		var s []byte
		if c.len >= 0 {
			s = make([]byte, c.len)
		}
		if got := ClampPos(s, c.pos); got != c.res {
			t.Errorf("ClampPos(%d, %d) should be %d, not %d", c.len, c.pos, c.res, got)
		}
	}

	test(Case{-1, -1, 0})
	test(Case{-1, 0, 0})
	test(Case{-1, 1, 0})

	test(Case{0, -1, 0})
	test(Case{0, 0, 0})
	test(Case{0, 1, 0})

	test(Case{1, -1, 0})
	test(Case{1, 0, 0})
	test(Case{1, 1, 1})

	test(Case{2, -1, 0})
	test(Case{2, 0, 0})
	test(Case{2, 1, 1})

	test(Case{3, -1, 0})
	test(Case{3, 3, 3})
	test(Case{3, 4, 3})

	test(Case{4, -10, 0})
	test(Case{4, 0, 0})
	test(Case{4, 10, 4})
}
