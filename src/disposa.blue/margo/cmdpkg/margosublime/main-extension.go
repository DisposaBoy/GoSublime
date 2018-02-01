// +build margo_extension

package margosublime

import (
	"margo"
)

func init() {
	margoExt = margo.Margo
}
