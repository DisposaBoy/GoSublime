package golang

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"io"
	"io/ioutil"
	"margo.sh/cmdpkg/margo/cmdrunner"
	"margo.sh/mg"
	yotsuba "margo.sh/why_would_you_make_yotsuba_cry"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
)

var (
	fnPosPat = regexp.MustCompile(`^(.+):(\d+):(\d+)$`)
)

type Guru struct {
	mg.ReducerType
}

func (g *Guru) RCond(mx *mg.Ctx) bool {
	return mx.LangIs(mg.Go)
}

func (g *Guru) RMount(mx *mg.Ctx) {
	go cmdrunner.Cmd{
		Name:     "go",
		Args:     []string{"install", "margo.sh/vendor/golang.org/x/tools/cmd/guru"},
		Env:      yotsuba.AgentBuildEnv,
		OutToErr: true,
	}.Run()
}

func (g *Guru) Reduce(mx *mg.Ctx) *mg.State {
	switch act := mx.Action.(type) {
	case mg.QueryUserCmds:
		return mx.AddUserCmds(
			mg.UserCmd{
				Title: "Guru Definition",
				Name:  "guru.definition",
				Desc:  "show declaration of selected identifier",
			},
		)
	case mg.RunCmd:
		return g.runCmd(mx, act)
	default:
		return mx.State
	}
}

func (g *Guru) runCmd(mx *mg.Ctx, rc mg.RunCmd) *mg.State {
	if rc.Name == "goto.definition" || rc.Name == "guru.definition" {
		return mx.AddBuiltinCmds(mg.BuiltinCmd{Name: rc.Name, Run: g.runDef})
	}

	if rc.Name != mg.RcActuate || rc.StringFlag("button", "left") != "left" {
		return mx.State
	}

	cx := NewViewCursorCtx(mx)
	var onId *ast.Ident
	var onSel *ast.SelectorExpr
	if !cx.Set(&onId) && !cx.Set(&onSel) {
		// we're not on a name, nothing to look for
		return mx.State
	}
	// we're on a func decl name, we're already at the definition
	if nm, _ := cx.FuncDeclName(); nm != "" {
		return mx.State
	}

	return mx.AddBuiltinCmds(mg.BuiltinCmd{Name: rc.Name, Run: g.runDef})
}

func (g *Guru) runDef(cx *mg.CmdCtx) *mg.State {
	go g.definition(cx)
	return cx.State
}

func (g *Guru) definition(bx *mg.CmdCtx) {
	defer bx.Output.Close()
	defer bx.Begin(mg.Task{Title: "guru definition", ShowNow: true}).Done()

	v := bx.View
	dir := v.Dir()
	fn := v.Filename()

	if v.Path == "" {
		tmpDir, err := ioutil.TempDir("", "guru")
		if err == nil {
			defer os.RemoveAll(tmpDir)
			fn = filepath.Join(tmpDir, v.Name)
			src, _ := v.ReadAll()
			ioutil.WriteFile(fn, src, 0600)
		}
	}

	cmd := exec.Command(
		"guru",
		"-json",
		"-tags", g.wasmTags(bx.Ctx),
		"-modified",
		"definition",
		fmt.Sprintf("%s:#%d", fn, v.Pos),
	)
	cmd.Dir = dir
	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = bx.Output
	cmd.Env = bx.Env.Environ()
	if v.Dirty {
		src, _ := v.ReadAll()
		hdr := &bytes.Buffer{}
		fmt.Fprintf(hdr, "%s\n%d\n", fn, len(src))
		cmd.Stdin = io.MultiReader(hdr, bytes.NewReader(src))
	}

	if err := cmd.Run(); err != nil {
		fmt.Fprintln(bx.Output, "Error:", err)
		return
	}

	res := struct {
		ObjPos string `json:"objpos"`
	}{}
	if err := json.Unmarshal(buf.Bytes(), &res); err != nil {
		fmt.Fprintln(bx.Output, "cannot decode guru output:", err)
	}

	m := fnPosPat.FindStringSubmatch(res.ObjPos)
	if len(m) != 4 {
		fmt.Fprintln(bx.Output, "cannot parse guru objpos:", res.ObjPos)
		return
	}

	n := func(s string) int {
		n, _ := strconv.Atoi(s)
		if n > 0 {
			return n - 1
		}
		return 0
	}

	fn = m[1]
	if v.Path == "" && filepath.Base(fn) == v.Name {
		fn = v.Name
	}
	bx.Store.Dispatch(mg.Activate{
		Path: fn,
		Row:  n(m[2]),
		Col:  n(m[3]),
	})
}

func (g *Guru) wasmTags(mx *mg.Ctx) string {
	tags := "js wasm"
	sysjs := "syscall/js"
	v := mx.View
	src, _ := v.ReadAll()
	if len(src) == 0 {
		return ""
	}

	pf := ParseFile(mx.Store, v.Filename(), src)
	for _, spec := range pf.AstFile.Imports {
		p := spec.Path
		if p == nil {
			continue
		}
		if s, _ := strconv.Unquote(p.Value); s == sysjs {
			return tags
		}
	}
	if v.Path == "" {
		// file doesn't exist, so there's no package
		return ""
	}

	pkg, _ := BuildContext(mx).ImportDir(mx.View.Dir(), 0)
	if pkg == nil {
		return ""
	}
	for _, l := range [][]string{pkg.Imports, pkg.TestImports} {
		for _, s := range l {
			if s == sysjs {
				return tags
			}
		}
	}

	return ""
}
