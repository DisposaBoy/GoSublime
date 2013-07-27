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
	return mPkgPathsRes(m.Env, m.Exclude), ""
}

func init() {
	registry.Register("pkgpaths", func(_ *Broker) Caller {
		return &mPkgPaths{}
	})
}

func mPkgPathsRes(env map[string]string, exclude []string) map[string]map[string]string {
	lck := sync.Mutex{}
	goroot, gopaths := envRootList(env)
	res := map[string]map[string]string{}

	wg := sync.WaitGroup{}
	proc := func(srcDir string) {
		wg.Add(1)
		go func() {
			defer wg.Done()

			paths := pkgPaths(srcDir, exclude)
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

	return res
}
