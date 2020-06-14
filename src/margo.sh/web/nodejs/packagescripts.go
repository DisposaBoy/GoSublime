package nodejs

import (
	"encoding/json"
	"margo.sh/mg"
	"os"
	"sort"
)

// PackageScripts adds UserCmd entries for each script defined in package.json
type PackageScripts struct {
	mg.ReducerType

	// Cmd is the command to run i.e. `yarn` or `npm`
	// if not set and `yarn.lock` in found in the package root, `yarn` will be used,
	// otherwise `npm` is use.
	Cmd string
}

func (ps *PackageScripts) RCond(mx *mg.Ctx) bool {
	return mx.ActionIs(mg.QueryUserCmds{})
}

func (ps *PackageScripts) cmd(mx *mg.Ctx, root string) string {
	if ps.Cmd != "" {
		return ps.Cmd
	}
	if _, _, err := mx.VFS.Poke(root).Locate("yarn.lock"); err == nil {
		return "yarn"
	}
	return "npm"
}

func (ps *PackageScripts) Reduce(mx *mg.Ctx) *mg.State {
	p := struct{ Scripts map[string]string }{}
	root, ok := ps.scanPkgJSON(mx, mx.View.Dir(), &p)
	if !ok || len(p.Scripts) == 0 {
		return mx.State
	}

	cmds := make(mg.UserCmdList, 0, len(p.Scripts))
	cmd := ps.cmd(mx, root)
	for name, script := range p.Scripts {
		cmds = append(cmds, mg.UserCmd{
			Title: cmd + " run " + name,
			Desc:  script,
			Name:  cmd,
			Args:  []string{"run", name},
		})
	}
	sort.Sort(cmds)
	return mx.AddUserCmds(cmds...)
}

func (ps *PackageScripts) scanPkgJSON(mx *mg.Ctx, dir string, p interface{}) (root string, ok bool) {
	nd, _, err := mx.VFS.Poke(dir).Locate("package.json")
	if err != nil {
		return "", false
	}
	f, err := os.Open(nd.Path())
	if err != nil {
		return "", false
	}
	defer f.Close()
	err = json.NewDecoder(f).Decode(p)
	return nd.Parent().Path(), err == nil
}
