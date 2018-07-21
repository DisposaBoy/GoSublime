package mg

import (
	"io/ioutil"
	"regexp"
)

var (
	sanitizeDirNamePat = regexp.MustCompile(`[^-~,.\w]`)
)

func MkTempDir(name string) (string, error) {
	return ioutil.TempDir("", ".margo~~"+sanitizeDirNamePat.ReplaceAllString(name, "~")+"~~")
}
