package pkglst

import (
	"margo.sh/golang/goutil"
	"margo.sh/internal/vfs"
	"margo.sh/mg"
	"path/filepath"
)

func ImportDir(mx *mg.Ctx, path string) (*Pkg, error) {
	path = filepath.Clean(path)

	kv, _, err := vfs.Root.StatKV(path)
	if err != nil {
		return nil, err
	}

	type K struct{ path string }
	k := K{path}
	if p, ok := kv.Get(k).(*Pkg); ok {
		return p, nil
	}

	bpkg, err := goutil.BuildContext(mx).ImportDir(path, 0)
	if err != nil {
		return nil, err
	}

	p := &Pkg{
		Dir:        bpkg.Dir,
		Name:       bpkg.Name,
		ImportPath: bpkg.ImportPath,
		Standard:   bpkg.Goroot,
	}
	p.finalize()
	kv.Put(k, p)
	return p, nil
}
