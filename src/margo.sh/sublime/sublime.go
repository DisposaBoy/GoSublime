package sublime

import (
	"bytes"
	"fmt"
	"github.com/urfave/cli"
	"go/build"
	"io/ioutil"
	"margo.sh/cmdpkg/margo/cmdrunner"
	"margo.sh/mg"
	"margo.sh/mgcli"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	AgentName = "margo.sublime"
)

var (
	Commands = mgcli.Commands{
		Name: AgentName,
		Build: &cli.Command{
			Action: mgcli.Action(buildAction),
		},
		Run: &cli.Command{
			SkipFlagParsing: true,
			SkipArgReorder:  true,
			Action:          mgcli.Action(runAction),
		},
	}

	logger = mg.NewLogger(os.Stderr)
)

func buildAction(c *cli.Context) error {
	tags := "margo"
	pkg := extensionPkg()
	if pkg != nil {
		fixExtPkg(pkg)
		tags = "margo margo_extension"
	}
	err := goInstallAgent(os.Getenv("MARGO_SUBLIME_GOPATH"), tags)
	if err != nil {
		return fmt.Errorf("check console for errors: %s", err)
	}
	return nil
}

func runAction(c *cli.Context) error {
	name := AgentName
	if exe, err := exec.LookPath(name); err == nil {
		name = exe
	}
	return cmdrunner.Cmd{Name: name, Args: c.Args()}.Run()
}

func goInstallAgent(gp string, tags string) error {
	args := []string{"install", "-v", "-tags=" + tags}
	if os.Getenv("MARGO_BUILD_FLAGS_RACE") == "1" {
		args = append(args, "-race")
	}
	for _, tag := range build.Default.ReleaseTags {
		if tag == "go1.10" {
			args = append(args, "-i")
			break
		}
	}
	args = append(args, "margo.sh/cmd/"+AgentName)
	cr := cmdrunner.Cmd{
		Name:     "go",
		Args:     args,
		OutToErr: true,
	}
	if gp != "" {
		cr.Env = map[string]string{"GOPATH": gp}
	}
	return cr.Run()
}

func extensionPkg() *build.Package {
	pkg, err := build.Import("margo", "", 0)
	if err != nil || len(pkg.GoFiles) == 0 {
		return nil
	}
	return pkg
}

func fixExtPkg(pkg *build.Package) {
	for _, fn := range pkg.GoFiles {
		fixExtFile(filepath.Join(pkg.Dir, fn))
	}
}

func fixExtFile(fn string) {
	p, err := ioutil.ReadFile(fn)
	if err != nil {
		logger.Println("fixExtFile:", err)
		return
	}

	from := `disposa.blue/margo`
	to := `margo.sh`
	q := bytes.Replace(p, []byte(from), []byte(to), -1)
	if bytes.Equal(p, q) {
		return
	}

	bak := fn + ".~mgfix~.bak"
	errOk := func(err error) string {
		if err != nil {
			return err.Error()
		}
		return "ok"
	}

	logger.Printf("mgfix %s: replace `%s` -> `%s`\n", fn, from, to)
	err = os.Rename(fn, bak)
	logger.Printf("mgfix %s: rename -> `%s`: %s\n", fn, bak, errOk(err))
	if err == nil {
		err := ioutil.WriteFile(fn, q, 0644)
		logger.Printf("mgfix %s: saving: %s\n", fn, errOk(err))
	}
}
