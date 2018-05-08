package golang

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"margo.sh/cmdpkg/margo/cmdrunner"
	"margo.sh/mg"
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

func (g *Guru) ReducerCond(mx *mg.Ctx) bool {
	return mx.LangIs(mg.Go)
}

func (g *Guru) ReducerMount(mx *mg.Ctx) {
	go cmdrunner.Cmd{
		Name:     "go",
		Args:     []string{"install", "margo.sh/vendor/golang.org/x/tools/cmd/guru"},
		OutToErr: true,
	}.Run()
}

func (g *Guru) Reduce(mx *mg.Ctx) *mg.State {
	switch mx.Action.(type) {
	case mg.QueryUserCmds:
		return mx.AddUserCmds(
			mg.UserCmd{
				Title: "Guru Definition",
				Name:  "guru.definition",
				Desc:  "show declaration of selected identifier",
			},
		)
	case mg.RunCmd:
		runDef := func(bx *mg.BultinCmdCtx) *mg.State {
			go g.definition(bx)
			return bx.State
		}
		return mx.AddBuiltinCmds(
			mg.BultinCmd{Name: "goto.definition", Run: runDef},
			mg.BultinCmd{Name: "guru.definition", Run: runDef},
		)
	default:
		return mx.State
	}
}

func (g *Guru) definition(bx *mg.BultinCmdCtx) {
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
		"/user/gp/bin/guru",
		"-json",
		"-modified",
		"definition",
		fmt.Sprintf("%s:#%d", fn, v.Pos),
	)
	cmd.Dir = dir
	buf := &bytes.Buffer{}
	cmd.Stdout = buf
	cmd.Stderr = bx.Output
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
