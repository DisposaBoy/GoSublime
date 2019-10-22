package mgutil

import (
	"path"
	"path/filepath"
	"strings"
	"unicode"
)

var (
	// ShortFnEnv is the list of envvars that are used in ShortFn.
	ShortFnEnv = []string{
		"GOPATH",
		"GOROOT",
	}
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

// ShortFn returns a shortened form of filename fn for display in UIs.
//
// If env is set, it's used to override os.Getenv.
//
// The following prefix/ replacements are made (in the listed order):
// * Envvar names listed in ShortFnEnv are replaced with `$name/`.
// * `HOME` or `USERPROFILE` envvars are replaced with `~/`.
//
// All other (non-prefix) path components except the last 2 are replaced with their first letter and preceding dots.
//
// This mimics the similar path display feature in shells like Fish.
//
// e.g. Given a long path like `/home/user/.config/sublime-text-3/Packages/User/GoSublime/pkg/mod/github.com/DisposaBoy/pkg@v1.23/go.mod`:
// * Given `$GOPATH=/home/user/.config/sublime-text-3/Packages/User/GoSublime`,
//   `$GOPATH/p/m/g/D/pkg@v1.2.3/go.mod`
// * Otherwise, `~/.c/s/P/U/G/p/m/g/D/pkg@v1.2.3/go.mod` is returned.
func ShortFn(fn string, env EnvMap) string {
	return shortFn(fn, env.Getenv)
}

func shortFn(fn string, getenv func(k string, def string) string) string {
	repl := shortFnRepl(getenv)
	fn = repl.Replace(filepath.Clean(fn))
	l := strings.Split(fn, string(filepath.Separator))
	if len(l) <= 3 {
		return fn
	}
	for i, s := range l[:len(l)-2] {
		if strings.HasPrefix(s, "~") || strings.HasPrefix(s, "$") {
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

// ShortFilename calls ShortFn(fn, nil)
func ShortFilename(fn string) string {
	return ShortFn(fn, nil)
}

func shortFnRepl(getenv func(k string, def string) string) *strings.Replacer {
	const sep = string(filepath.Separator)
	l := []string{}
	for _, k := range ShortFnEnv {
		for _, s := range PathList(getenv(k, "")) {
			l = append(l, s+sep, "$"+k+sep)
		}
	}
	for _, k := range []string{"HOME", "USERPROFILE"} {
		if s := getenv(k, ""); s != "" {
			l = append(l, s+sep, "~"+sep)
		}
	}
	return strings.NewReplacer(l...)
}
