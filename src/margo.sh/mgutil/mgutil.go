// Package mgutil is a collections of utility types and functions with no dependency on margo.sh/mg
package mgutil

import (
	"strconv"
	"strings"
)

// QuoteCmdArg uses strconv.Quote to quote the command arg s.
// NOTE: the result is for display only, and should not be used for shell security.
// e.g.
// `a b c` -> `"a b c"`
// `abc` -> `abc`
// `-abc=123` -> `-abc=123`
// `-abc=1 2 3` -> `-abc="1 2 3"`
func QuoteCmdArg(s string) string {
	eqPos := strings.Index(s, "=")
	switch {
	case s == "":
		return `""`
	case !strings.Contains(s, " "):
		return s
	case strings.HasPrefix(s, "-") && eqPos > 0:
		return s[:eqPos+1] + strconv.Quote(s[eqPos+1:])
	default:
		return strconv.Quote(s)
	}
}

// QuoteCmd joins `name [args]` with name and each arg quoted with QuoteCmdArg
// NOTE: the result is for display only, and should not be used for shell security.
func QuoteCmd(name string, args ...string) string {
	a := append([]string{name}, args...)
	for i, s := range a {
		a[i] = QuoteCmdArg(s)
	}
	return strings.Join(a, " ")
}

// Clamp limits n to the interval [ lo, hi ]
func Clamp(lo, hi int, n int) int {
	switch {
	case n <= lo:
		return lo
	case n >= hi:
		return hi
	default:
		return n
	}
}

// ClampPos limits pos to the interval [ 0, len(s) ]
func ClampPos(s []byte, pos int) int {
	return Clamp(0, len(s), pos)
}
