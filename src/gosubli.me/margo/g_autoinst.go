package main

import (
	"go/parser"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	autoInstCh = make(chan AutoInstOptions, 10)
)

func autoInstall(ao AutoInstOptions) {
	select {
	case autoInstCh <- ao:
	default:
	}
}

type AutoInstOptions struct {
	// if ImportPaths is empty, Src is parsed in order to populate it
	ImportPaths []string
	Src         string

	// the environment variables as passed by the client - they should not be merged with os.Environ(...)
	// GOPATH is be valid
	Env map[string]string
}

func (a *AutoInstOptions) imports() map[string]string {
	m := map[string]string{}

	if len(a.ImportPaths) == 0 {
		_, af, _ := parseAstFile("a.go", a.Src, parser.ImportsOnly)
		a.ImportPaths = fileImportPaths(af)
	}

	for _, p := range a.ImportPaths {
		m[p] = filepath.FromSlash(p) + ".a"
	}

	return m
}

func (a *AutoInstOptions) install() {
	if a.Env == nil {
		a.Env = map[string]string{}
	}

	osArch := runtime.GOOS + "_" + runtime.GOARCH
	roots := []string{}

	if p := a.Env["GOROOT"]; p != "" {
		roots = append(roots, filepath.Join(p, "pkg", osArch))
	}

	psep := string(filepath.ListSeparator)
	if s := a.Env["_pathsep"]; s != "" {
		psep = s
	}

	for _, p := range strings.Split(a.Env["GOPATH"], psep) {
		if p != "" {
			roots = append(roots, filepath.Join(p, "pkg", osArch))
		}
	}

	if len(roots) == 0 {
		return
	}

	archiveOk := func(fn string) bool {
		for _, root := range roots {
			_, err := os.Stat(filepath.Join(root, fn))
			if err == nil {
				return true
			}
		}
		return false
	}

	el := envSlice(a.Env)
	installed := []string{}

	for path, fn := range a.imports() {
		if !archiveOk(fn) {
			cmd := exec.Command("go", "install", path)
			cmd.Env = el
			cmd.Stderr = ioutil.Discard
			cmd.Stdout = ioutil.Discard
			cmd.Run()

			if archiveOk(fn) {
				installed = append(installed, path)
			}
		}
	}

	if len(installed) > 0 {
		postMessage("auto-installed: %v", strings.Join(installed, ", "))
	}
}

func init() {
	go func() {
		for ao := range autoInstCh {
			ao.install()
		}
	}()
}
