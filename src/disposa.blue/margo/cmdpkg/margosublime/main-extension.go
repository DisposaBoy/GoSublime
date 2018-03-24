// +build margo_extension

package margosublime

import (
	// we don't really care what the declared package name is
	margo "margo"
)

func init() {
	margoExt = margo.Margo
	sublCfg = sublCfg.EnabledForLangs("*")
}
