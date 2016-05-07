package gocode

import (
	"fmt"
	"go/build"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var Margo = newMargoState()

type MargoConfig struct {
	Builtins      bool
	InstallSuffix string
	GOROOT        string
	GOPATHS       []string
}

type margoEnv struct {
	LibPath       string
	GOOS          string
	GOARCH        string
	Compiler      string
	GOROOT        string
	GOPATH        string
	InstallSuffix string
}

func (m *margoEnv) assign(c *config, p *package_lookup_context) {
	c.LibPath = m.LibPath

	p.GOOS = m.GOOS
	p.GOARCH = m.GOARCH
	p.Compiler = m.Compiler
	p.GOROOT = m.GOROOT
	p.GOPATH = m.GOPATH
	p.InstallSuffix = m.InstallSuffix
}

type margoState struct {
	sync.Mutex

	ctx       *auto_complete_context
	env       *package_lookup_context
	pkgCache  package_cache
	declCache *decl_cache
	prevEnv   margoEnv
	prevPkg   string
}

type MargoCandidate struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Class string `json:"class"`
}

func newMargoState() *margoState {
	env := &package_lookup_context{}
	pkgCache := new_package_cache()
	declCache := new_decl_cache(env)
	return &margoState{
		ctx:       new_auto_complete_context(pkgCache, declCache),
		env:       env,
		pkgCache:  pkgCache,
		declCache: declCache,
	}
}

func (m *margoState) Complete(c MargoConfig, file []byte, filename string, cursor int) []MargoCandidate {
	m.Lock()
	defer m.Unlock()

	m.updateConfig(c, filename)

	list, _ := m.ctx.apropos(file, filename, cursor)
	candidates := make([]MargoCandidate, len(list))
	for i, c := range list {
		candidates[i] = MargoCandidate{
			Name:  c.Name,
			Type:  c.Type,
			Class: c.Class.String(),
		}
	}
	return candidates
}

func (m *margoState) updateConfig(c MargoConfig, filename string) {
	pl := []string{}
	osArch := runtime.GOOS + "_" + runtime.GOARCH
	if c.InstallSuffix != "" {
		osArch += "_" + c.InstallSuffix
	}
	add := func(p string) {
		if p != "" {
			pl = append(pl, filepath.Join(p, "pkg", osArch))
		}
	}

	add(c.GOROOT)
	for _, p := range c.GOPATHS {
		add(p)
	}

	sep := string(filepath.ListSeparator)

	nv := margoEnv{
		LibPath:       strings.Join(pl, sep),
		GOOS:          runtime.GOOS,
		GOARCH:        runtime.GOARCH,
		Compiler:      runtime.Compiler,
		GOROOT:        c.GOROOT,
		GOPATH:        strings.Join(c.GOPATHS, sep),
		InstallSuffix: c.InstallSuffix,
	}
	nv.assign(&g_config, m.env)
	g_config.ProposeBuiltins = c.Builtins
	m.env.CurrentPackagePath = ""
	p, _ := m.env.ImportDir(filepath.Dir(filename), build.FindOnly)
	if p != nil {
		m.env.CurrentPackagePath = p.ImportPath
	}
	if m.prevPkg != m.env.CurrentPackagePath {
		m.prevPkg = m.env.CurrentPackagePath
		fmt.Fprintf(os.Stderr, "Gocode pkg: %#v\n", m.env.CurrentPackagePath)
	}
	if m.prevEnv != nv {
		m.prevEnv = nv
		fmt.Fprintf(os.Stderr, "Gocode env: %#v\n", nv)
	}
}
