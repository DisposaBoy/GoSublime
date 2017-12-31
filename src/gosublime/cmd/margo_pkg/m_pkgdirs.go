package margo_pkg

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

var (
	pkgDirsLck   = sync.RWMutex{}
	pkgDirsCache = map[string]bool{}
)

type mPkgDirs struct {
	Env map[string]string
}

func (m *mPkgDirs) Call() (interface{}, string) {
	return pkgDirs(m.Env), ""
}

func init() {
	registry.Register("pkg_dirs", func(_ *Broker) Caller {
		return &mPkgDirs{
			Env: map[string]string{},
		}
	})
}

func pkgDirs(env map[string]string) map[string]map[string]string {
	res := map[string]map[string]string{}
	for _, root := range rootDirs(env) {
		res[root] = map[string]string{}
		walkRootDir(root, res[root], root)
	}
	return res
}

func walkRootDir(root string, m map[string]string, basePath string) {
	dir, err := os.Open(root)
	if err != nil {
		return
	}
	defer dir.Close()

	importPath, err := filepath.Rel(basePath, root)
	if err != nil {
		importPath = root
	}
	importPath = path.Clean(filepath.ToSlash(importPath))
	idealName := path.Base(importPath) + ".go"

	names, err := dir.Readdirnames(-1)
	for _, name := range names {
		if name[0] == '.' || name[0] == '_' {
			continue
		}

		fn := filepath.Join(root, name)
		if strings.HasSuffix(name, ".go") {
			isIdeal := false
			oldFn, ok := m[importPath]
			if ok {
				isIdeal = strings.HasSuffix(oldFn, idealName)
			}
			if !ok || name == idealName || (!isIdeal && name == "main.go") {
				m[importPath] = fn
			}
		} else {
			pkgDirsLck.RLock()
			isDir, ok := pkgDirsCache[fn]
			pkgDirsLck.RUnlock()

			if ok {
				if isDir {
					walkRootDir(fn, m, basePath)
				}
			} else if fi, err := os.Lstat(fn); err == nil {
				pkgDirsLck.Lock()
				pkgDirsCache[fn] = fi.IsDir()
				pkgDirsLck.Unlock()

				if fi.IsDir() {
					walkRootDir(fn, m, basePath)
				}
			}
		}
	}
}
