package mgutil

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"unicode"
)

var (
	homeDirPfx = func() string {
		for _, k := range []string{"HOME", "USERPROFILE"} {
			if s := filepath.Clean(os.Getenv(k)); filepath.IsAbs(s) {
				return s + string(filepath.Separator)
			}
		}
		return ""
	}()
	homeDirTilde = "~" + string(filepath.Separator)
)

// FilePathParent returns the parent(filepath.Dir) of fn if it has a parent.
// If fn has no parent, an empty string is returned instead of ".", "/" or fn itself.
func FilePathParent(fn string) string {
	fn = filepath.Clean(fn)
	dir := filepath.Dir(fn)
	if dir == "." || dir == fn || dir == string(filepath.Separator) {
		return ""
	}
	return dir
}

// PathParent returns the parent(path.Dir) of fn if it has a parent.
// If fn has no parent, an empty string is returned instead of "." or fn itself.
func PathParent(fn string) string {
	fn = path.Clean(fn)
	dir := path.Dir(fn)
	if dir == "." || dir == fn || dir == "/" {
		return ""
	}
	return dir
}

// ShortFilename returns a shortened form of fn for display in UIs.
//
// This mimicks the similar path display feature in shells like Fish.
// Given a long path like `/home/user/.config/sublime-text-3/Packages/User/GoSublime/pkg/mod/github.com/DisposaBoy/pkg@v1.23/go.mod`,
// it returns `~/.c/s/P/U/G/p/m/g/D/pkg@v1.2.3/go.mod`
func ShortFilename(fn string) string {
	fn = filepath.Clean(fn)
	if s := strings.TrimPrefix(fn, homeDirPfx); s != fn {
		fn = homeDirTilde + s
	}
	l := strings.Split(fn, string(filepath.Separator))
	if len(l) <= 3 {
		return fn
	}
	for i, s := range l[:len(l)-2] {
		if s == homeDirTilde {
			continue
		}
		for j, r := range s {
			if unicode.IsLetter(r) {
				l[i] = s[:j] + string(r)
				break
			}
		}
	}
	return strings.Join(l, string(filepath.Separator))
}
