// +build margo_extension

package margosublime

import (
	"margo"
)

func init() {
	initFuncs = append(initFuncs, margo.Init)
}
