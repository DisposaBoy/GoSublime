// +build !windows

package mgutil

import (
	"path/filepath"
	"testing"
)

func TestShortFn(t *testing.T) {
	home := "/home/user"
	gp := home + "/.config/sublime-text-3/Packages/User/GoSublime"
	fn := gp + "/pkg/mod/github.com/DisposaBoy/pkg@v1.2.3/go.mod"
	tbl := []struct {
		nm  string
		fn  string
		res string
		env func(string, string) string
	}{
		{
			nm:  "With $HOME",
			fn:  fn,
			res: `~/.c/s/P/U/G/p/m/g/D/pkg@v1.2.3/go.mod`,
			env: (EnvMap{"HOME": home}).Get,
		},
		{
			nm:  "Without $HOME",
			fn:  fn,
			res: `/h/u/.c/s/P/U/G/p/m/g/D/pkg@v1.2.3/go.mod`,
			env: (EnvMap{"HOME": ""}).Get,
		},
		{
			nm:  "With $GOPATH",
			fn:  fn,
			res: `$GOPATH/p/m/g/D/pkg@v1.2.3/go.mod`,
			env: (EnvMap{"GOPATH": gp}).Get,
		},
	}
	for _, r := range tbl {
		r.fn = filepath.FromSlash(r.fn)
		r.res = filepath.FromSlash(r.res)
		t.Run(r.nm, func(t *testing.T) {
			res := shortFn(fn, r.env)
			if res != r.res {
				t.Errorf("ShortFn(`%s`) = `%s`. Expected `%s`.", r.fn, res, r.res)
			}
		})
	}
}
