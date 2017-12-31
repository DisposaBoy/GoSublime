package margo

import (
	"go/build"
	"sync"
)

var (
	opts = struct {
		sync.RWMutex
		o Opts
	}{}
)

// PathFilterFunc returns true if path should not be ignore (when walking GOPATH, etc.)
type PathFilterFunc func(path string) bool

// ImportPathsFunc returns a map of package *import path* to title for display to the user
type ImportPathsFunc func(srcDir string, ctx *build.Context) map[string]string

// Opts contains options used by margo and its methods
type Opts struct {
	ImportPaths ImportPathsFunc
}

// Configure calls each func in funcs to configure the shared Opts
func Configure(funcs ...func(*Opts)) {
	opts.Lock()
	defer opts.Unlock()

	for _, f := range funcs {
		f(&opts.o)
	}
}

// Options returns a copy of the shared Opts
func Options() Opts {
	opts.RLock()
	defer opts.RUnlock()

	return opts.o
}
