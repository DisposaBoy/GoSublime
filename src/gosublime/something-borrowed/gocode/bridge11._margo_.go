// +build go1.11

package gocode

import (
	"fmt"
	"os"
)

func init() {
	margoGocodeEnabled = false
	fmt.Fprintln(os.Stderr, "margo: nsf/gocode is not enabled in go1.11. See https://margo.sh/b/migrate/_r=gs")
}
