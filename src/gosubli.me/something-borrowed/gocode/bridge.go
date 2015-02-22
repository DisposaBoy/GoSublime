package gocode

import (
	"go/build"
	"path/filepath"
	"strings"
	"sync"
)

var Margo = newMgoDaemon()

type MargoCandidate struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Class string `json:"class"`
}

type MargoConfig struct {
	GOROOT        string
	GOPATH        string
	InstallSuffix string
	Builtins      bool // propose builtin functions
}

type mgoDaemon struct {
	autocomplete *auto_complete_context
	declcache    *decl_cache
	pkgcache     package_cache
	context      build.Context
	mu           sync.Mutex
}

// newMgoDaemon returns a new mgoDaemon.
func newMgoDaemon() *mgoDaemon {
	m := mgoDaemon{
		pkgcache: new_package_cache(),
		context:  build.Default,
		mu:       sync.Mutex{},
	}
	m.declcache = new_decl_cache(m.context)
	m.autocomplete = new_auto_complete_context(m.pkgcache, m.declcache)
	return &m
}

func (m *mgoDaemon) Complete(file []byte, filename string, cursor int, config MargoConfig) []MargoCandidate {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch {
	case m.context.GOROOT != config.GOROOT:
		m.update(config)
	case m.context.GOPATH != config.GOPATH:
		m.update(config)
	case m.context.InstallSuffix != config.InstallSuffix:
		m.update(config)
	case g_config.ProposeBuiltins != config.Builtins:
		g_config.ProposeBuiltins = config.Builtins
	}
	list, _ := m.autocomplete.apropos(file, filename, cursor)
	if list == nil {
		return []MargoCandidate{}
	}
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

// update, Updates mgoDaemon and g_config path variables.
func (m *mgoDaemon) update(c MargoConfig) {
	m.context.GOROOT = c.GOROOT
	m.context.GOPATH = c.GOPATH
	m.context.InstallSuffix = c.InstallSuffix

	g_config.ProposeBuiltins = c.Builtins
	g_config.LibPath = m.libPath()

	m.pkgcache = new_package_cache()
	m.declcache = new_decl_cache(m.context)
	m.autocomplete = new_auto_complete_context(m.pkgcache, m.declcache)
}

// libPath returns a list of Go package directories
func (m *mgoDaemon) libPath() string {
	const sep = string(filepath.ListSeparator)
	var all []string
	arch := m.osArch()
	if m.context.GOROOT != "" {
		dir := filepath.Join(m.context.GOROOT, "pkg", arch)
		all = append(all, dir)
	}
	for _, p := range m.gopath() {
		dir := filepath.Join(p, "pkg", arch)
		all = append(all, dir)
	}
	return strings.Join(all, sep)
}

// osArch returns the os and arch specific package directory
func (m *mgoDaemon) osArch() string {
	if m.context.InstallSuffix == "" {
		return m.context.GOOS + "_" + m.context.GOARCH
	}
	return m.context.GOOS + "_" + m.context.GOARCH + "_" + m.context.InstallSuffix
}

// gopath returns the list of Go path directories.
func (m *mgoDaemon) gopath() []string {
	var all []string
	for _, p := range splitPathList(m.context.GOPATH) {
		if p != "" && p != m.context.GOROOT && !strings.HasPrefix(p, "~") {
			all = append(all, p)
		}
	}
	return all
}

func splitPathList(s string) []string {
	return filepath.SplitList(s)
}
