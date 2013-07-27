package main

import (
	"path/filepath"
	"sync"
)

type mPkgPaths struct {
	Env     map[string]string
	Exclude []string
}

func (m *mPkgPaths) Call() (interface{}, string) {
	lck := sync.Mutex{}
	goroot, gopaths := envRootList(m.Env)
	res := M{}

	wg := sync.WaitGroup{}
	proc := func(srcDir string) {
		wg.Add(1)
		go func() {
			defer wg.Done()

			paths := pkgPaths(srcDir, m.Exclude)
			if len(paths) > 0 {
				lck.Lock()
				res[srcDir] = paths
				lck.Unlock()
			}
		}()
	}

	proc(filepath.Join(goroot, "src", "pkg"))
	for _, p := range gopaths {
		proc(filepath.Join(p, "src"))
	}
	wg.Wait()

	return res, ""
}

func init() {
	registry.Register("pkgpaths", func(_ *Broker) Caller {
		return &mPkgPaths{}
	})
}
