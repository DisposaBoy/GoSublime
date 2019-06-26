package pkglst

import (
	"margo.sh/golang/gopkg"
	"margo.sh/mg"
	"margo.sh/vfs"
	"path/filepath"
	"sort"
	"sync"
)

type View struct {
	List         []*gopkg.Pkg
	ByDir        map[string]*gopkg.Pkg
	ByImportPath map[string][]*gopkg.Pkg
	ByName       map[string][]*gopkg.Pkg
}

func (vu View) shallowClone(lstLen int) View {
	x := View{
		ByDir:        make(map[string]*gopkg.Pkg, len(vu.ByDir)+lstLen),
		ByImportPath: make(map[string][]*gopkg.Pkg, len(vu.ByImportPath)+lstLen),
		ByName:       make(map[string][]*gopkg.Pkg, len(vu.ByName)+lstLen),
	}
	for k, p := range vu.ByDir {
		x.ByDir[k] = p
	}
	for k, l := range vu.ByImportPath {
		x.ByImportPath[k] = l[:len(l):len(l)]
	}
	for k, l := range vu.ByName {
		x.ByName[k] = l[:len(l):len(l)]
	}
	return x
}

func (vu View) PruneDir(dir string) View {
	dir = filepath.Clean(dir)
	p, exists := vu.ByDir[dir]
	if !exists {
		return vu
	}

	delpkg := func(m map[string][]*gopkg.Pkg, k string) {
		l := m[k]
		if len(l) == 0 {
			return
		}

		x := make([]*gopkg.Pkg, 0, len(l)-1)
		for _, p := range l {
			if p.Dir != dir {
				x = append(x, p)
			}
		}

		if len(x) == 0 {
			delete(m, k)
		} else {
			m[k] = x
		}
	}
	x := vu.shallowClone(0)
	delete(x.ByDir, p.Dir)
	delpkg(x.ByImportPath, p.ImportPath)
	delpkg(x.ByName, p.Name)
	return x
}

func (vu View) Add(lst ...*gopkg.Pkg) View {
	x := vu.shallowClone(len(lst))

	for _, p := range lst {
		x.ByDir[p.Dir] = p
		x.ByImportPath[p.ImportPath] = append(x.ByImportPath[p.ImportPath], p)
		x.ByName[p.Name] = append(x.ByName[p.Name], p)
	}

	x.List = make([]*gopkg.Pkg, 0, len(x.ByDir))
	for _, p := range x.ByDir {
		x.List = append(x.List, p)
	}
	sort.Slice(x.List, func(i, j int) bool {
		a, b := x.List[i], x.List[j]
		switch {
		case a.Name != b.Name:
			return a.Name < b.Name
		case a.ImportPath != b.ImportPath:
			return a.ImportPath < b.ImportPath
		default:
			return a.Dir < b.Dir
		}
	})

	return x
}

type Cache struct {
	mu   sync.RWMutex
	view View
}

func (cc *Cache) Scan(mx *mg.Ctx, dir string) (output []byte, _ error) {
	lst, out, err := cc.vfsList(mx, dir)
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.view = cc.view.Add(lst...)

	return out, err
}

func (cc *Cache) PruneDir(dir string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.view = cc.view.PruneDir(dir)
}

func (cc *Cache) Add(l ...gopkg.Pkg) {
	x := make([]*gopkg.Pkg, len(l))
	for i, p := range l {
		p.Finalize()
		x[i] = &p
	}

	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.view = cc.view.Add(x...)
}

func (cc *Cache) View() View {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	return cc.view
}

func (cc *Cache) vfsList(mx *mg.Ctx, dir string) ([]*gopkg.Pkg, []byte, error) {
	lst := []*gopkg.Pkg{}
	mx.VFS.Peek(dir).Branches(func(nd *vfs.Node) {
		if p, err := gopkg.ImportDirNd(mx, nd); err == nil {
			lst = append(lst, p)
		}
	})
	return lst, nil, nil
}
