package margo

import (
	"path/filepath"
)

// FilterPath returns true if path is *not* usually ignored by the Go tools (.*, _*, testdata)
func FilterPath(path string) bool {
	name := filepath.Base(path)
	return name != "" && name[0] != '.' && name[0] != '_' && name != "testdata"
}

// FilterPathExt returns true if path does *not* have an extension *known* to be commonly found in Go projects
func FilterPathExt(path string) bool {
	return !knownFileExts[filepath.Ext(path)]
}
