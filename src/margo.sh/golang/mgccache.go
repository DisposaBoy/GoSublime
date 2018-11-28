package golang

import (
	"go/types"
	"margo.sh/mgpf"
	"regexp"
	"sync"
	"time"
)

// mgcCacheKey is the key used for caching package imports
type mgcCacheKey struct {
	gsuPkgInfo

	// Source indicates whether the package was imported from source code
	Source bool
}

func (mck mgcCacheKey) fallback() mgcCacheKey {
	fbk := mck
	fbk.Source = !fbk.Source
	return fbk
}

type mgcCacheEnt struct {
	Key mgcCacheKey
	Pkg *types.Package
	Dur time.Duration
}

type mgcCache struct {
	sync.RWMutex
	m map[mgcCacheKey]mgcCacheEnt
}

func (mc *mgcCache) get(k mgcCacheKey) (mgcCacheEnt, bool) {
	mc.RLock()
	defer mc.RUnlock()

	e, ok := mc.m[k]
	if !ok {
		mctl.dbgf("cache.miss: %+v\n", k)
	}
	return e, ok
}

func (mc *mgcCache) put(e mgcCacheEnt) {
	if !e.Pkg.Complete() {
		mctl.dbgf("cache.put: not storing %+v, it's incomplete\n", e.Key)
		return
	}

	mc.Lock()
	defer mc.Unlock()

	mc.m[e.Key] = e
	mctl.dbgf("cache.put: %+v %s\n", e.Key, mgpf.D(e.Dur))
}

func (mc *mgcCache) del(k mgcCacheKey) {
	mc.Lock()
	defer mc.Unlock()

	if _, exists := mc.m[k]; !exists {
		return
	}

	delete(mc.m, k)
	mctl.dbgf("cache.del: %+v\n", k)
}

func (mc *mgcCache) prune(pats ...*regexp.Regexp) []mgcCacheEnt {
	ents := []mgcCacheEnt{}
	defer func() {
		for _, e := range ents {
			mctl.dbgf("cache.prune: %+v\n", e.Key)
		}
	}()

	mc.Lock()
	defer mc.Unlock()

	for _, e := range mc.m {
		for _, pat := range pats {
			if pat.MatchString(e.Key.Path) {
				ents = append(ents, e)
				delete(mc.m, e.Key)
			}
		}
	}

	return ents
}

func (mc *mgcCache) size() int {
	mc.RLock()
	defer mc.RUnlock()

	return len(mc.m)
}

func (mc *mgcCache) entries() []mgcCacheEnt {
	mc.RLock()
	defer mc.RUnlock()

	l := make([]mgcCacheEnt, 0, len(mc.m))
	for _, e := range mc.m {
		l = append(l, e)
	}
	return l
}

func (mc *mgcCache) forEach(f func(mgcCacheEnt) bool) {
	mc.RLock()
	defer mc.RUnlock()

	for _, e := range mc.m {
		if !f(e) {
			break
		}
	}
}
