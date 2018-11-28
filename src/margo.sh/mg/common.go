package mg

import (
	"margo.sh/mgutil"
)

type StrSet = mgutil.StrSet

type EnvMap = mgutil.EnvMap

// PathList is an alias of mgutil.PathList
func PathList(s string) []string {
	return mgutil.PathList(s)
}

// IsParentDir is an alias of mgutil.IsParentDir
func IsParentDir(parentDir, childPath string) bool {
	return mgutil.IsParentDir(parentDir, childPath)
}
