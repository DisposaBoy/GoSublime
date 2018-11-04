package nodejs

import (
	"encoding/json"
	"margo.sh/mg"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

// PackageScripts adds UserCmd entries for each script defined in package.json
type PackageScripts struct {
	mg.ReducerType

	// Cmd is the command to run i.e. `yarn` or `npm`
	// if not set and `yarn` in found in $PATH it will be set to `yarn`,
	// otherwise it will be set to `npm`
	Cmd string
}

func (ps *PackageScripts) RCond(mx *mg.Ctx) bool {
	return mx.ActionIs(mg.QueryUserCmds{})
}

func (ps *PackageScripts) RMount(mx *mg.Ctx) {
	if ps.Cmd == "" {
		ps.Cmd = ps.yarnOrNPM()
	}
}

func (ps *PackageScripts) yarnOrNPM() string {
	if _, err := exec.LookPath("yarn"); err == nil {
		return "yarn"
	}
	return "npm"
}

func (ps *PackageScripts) Reduce(mx *mg.Ctx) *mg.State {
	p := struct{ Scripts map[string]string }{}
	ps.scanPkgJSON(mx.View.Dir(), &p)
	if len(p.Scripts) == 0 {
		return mx.State
	}

	cmds := make(mg.UserCmdList, 0, len(p.Scripts))
	for name, script := range p.Scripts {
		cmds = append(cmds, mg.UserCmd{
			Title: ps.Cmd + " run " + name,
			Desc:  script,
			Name:  ps.Cmd,
			Args:  []string{"run", name},
		})
	}
	sort.Sort(cmds)
	return mx.AddUserCmds(cmds...)
}

func (ps *PackageScripts) scanPkgJSON(dir string, p interface{}) bool {
	fn := filepath.Join(dir, "package.json")
	f, err := os.Open(fn)
	if err == nil {
		defer f.Close()
		return json.NewDecoder(f).Decode(p) == nil
	}

	d := filepath.Dir(dir)
	if d == dir || !filepath.IsAbs(d) {
		return false
	}
	return ps.scanPkgJSON(d, p)
}
