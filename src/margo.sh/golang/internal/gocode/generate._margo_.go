// +build generate

package main

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if err := gen(); err != nil {
		log.Fatal(err)
	}
}

func gen() error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("Cannot get wd: %s", err)
	}

	defer func() {
		statusCmd := exec.Command("git", "status", wd)
		statusCmd.Stdin = os.Stdin
		statusCmd.Stdout = os.Stdout
		statusCmd.Stderr = os.Stderr
		statusCmd.Run()
	}()

	temp, err := ioutil.TempDir(wd, ".margo.")
	if err != nil {
		return fmt.Errorf("Cannot create temp dir: %s", err)
	}
	defer os.RemoveAll(temp)

	cloneDir := filepath.Join(temp, "gocode-temp")
	cloneCmd := exec.Command("git", "clone", "--depth", "1", "https://github.com/nsf/gocode", cloneDir)
	cloneCmd.Stdin = os.Stdin
	cloneCmd.Stdout = os.Stdout
	cloneCmd.Stderr = os.Stderr
	if err := cloneCmd.Run(); err != nil {
		return fmt.Errorf("Cannot clone gocode: %s", err)
	}

	origPkg, err := build.Default.ImportDir(wd, build.ImportComment)
	if err != nil {
		return fmt.Errorf("Cannot import bundled pakage: %s", err)
	}

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, cloneDir, nil, parser.ParseComments)
	bundlePkg := pkgs["main"]
	if err != nil || bundlePkg == nil {
		return fmt.Errorf("Cannot parse gocode pakage: %s", err)
	}
	save := func(fn string, af *ast.File) error {
		dst, err := os.Create(filepath.Join(wd, filepath.Base(fn)))
		if err != nil {
			return fmt.Errorf("Cannot create bundle file: %s", err)
		}
		defer dst.Close()

		if err := format.Node(dst, fset, af); err != nil {
			return fmt.Errorf("Cannot fmt file: %s: %s", fn, err)
		}
		return nil
	}

	for _, fn := range origPkg.GoFiles {
		if strings.HasSuffix(fn, "._margo_.go") {
			continue
		}
		if err := os.Remove(fn); err != nil {
			return fmt.Errorf("Cannot remove: %s: %s", fn, err)
		}
	}
	for fn, p := range bundlePkg.Files {
		p.Name = ast.NewIdent("gocode")
		if err := save(fn, p); err != nil {
			return err
		}
	}

	return nil
}
