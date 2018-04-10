// mgutil is a collections of utility types and functions with no dependency on margo.sh/mg
package mgutil

import (
	"strconv"
	"strings"
)

// QuoteFlag uses strconv.Quote to quote the command arg s.
// NOTE: It's intended for use in command display and should not be used for shell security.
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
