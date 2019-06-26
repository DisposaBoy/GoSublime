package gopkg

import (
	"bytes"
	"fmt"
	"github.com/rogpeppe/go-internal/modfile"
	"github.com/rogpeppe/go-internal/module"
	"github.com/rogpeppe/go-internal/semver"
	"go/build"
	"io/ioutil"
	"margo.sh/golang/goutil"
	"margo.sh/mg"
	"margo.sh/mgutil"
	"margo.sh/vfs"
	"os"
	"path/filepath"
	"strings"
)

func ScanFilter(de *vfs.Dirent) bool {
	nm := de.Name()
	if nm[0] == '.' || nm[0] == '_' || nm == "testdata" || nm == "node_modules" {
		return false
	}
	return de.IsDir() || strings.HasSuffix(nm, ".go")
}

func ImportDir(mx *mg.Ctx, dir string) (*Pkg, error) {
	return ImportDirNd(mx, mx.VFS.Poke(dir))
}

func ImportDirNd(mx *mg.Ctx, nd *vfs.Node) (*Pkg, error) {
	ls := nd.Ls().Filter(pkgNdFilter).Nodes()
	if len(ls) == 0 {
		return nil, &build.NoGoError{Dir: nd.Path()}
	}
	memo, err := nd.Memo()
	if err != nil {
		return nil, err
	}

	bctx := goutil.BuildContext(mx)
	type K struct{ GOROOT, GOPATH string }
	type V struct {
		p *Pkg
		e error
	}
	k := K{GOROOT: bctx.GOROOT, GOPATH: bctx.GOPATH}
	v := memo.Read(k, func() interface{} {
		p, err := importDirNd(mx, nd, bctx, ls)
		return V{p: p, e: err}
	}).(V)
	return v.p, v.e
}

func pkgNdFilter(nd *vfs.Node) bool {
	nm := nd.Name()
	return nm[0] != '.' && nm[0] != '_' &&
		strings.HasSuffix(nm, ".go") &&
		// there's no such thing as a ~~killer videotape~~go package with only test files
		!strings.HasSuffix(nm, "_test.go")
}

func importDirNd(mx *mg.Ctx, nd *vfs.Node, bctx *build.Context, ls []*vfs.Node) (*Pkg, error) {
	dir := nd.Path()
	var errNoGo error = &build.NoGoError{Dir: dir}
	bctx.IsDir = func(p string) bool {
		if p == dir {
			return true
		}
		return mx.VFS.IsDir(p)
	}
	bctx.ReadDir = func(p string) ([]os.FileInfo, error) {
		if p != dir {
			return mx.VFS.ReadDir(p)
		}
		if len(ls) == 0 {
			return nil, errNoGo
		}
		fi, err := ls[0].Stat()
		if err == nil {
			return []os.FileInfo{fi}, nil
		}
		return nil, err
	}
	resErr := errNoGo
	for len(ls) != 0 {
		bp, err := bctx.ImportDir(dir, 0)
		ls = ls[1:]
		if err != nil {
			resErr = err
			continue
		}
		p := &Pkg{
			Dir:        bp.Dir,
			Name:       bp.Name,
			ImportPath: bp.ImportPath,
			Goroot:     bp.Goroot,
		}
		p.Finalize()
		return p, nil
	}
	return nil, resErr
}

func FindPkg(mx *mg.Ctx, importPath, srcDir string) (*PkgPath, error) {
	bctx := goutil.BuildContext(mx)
	grDir := filepath.Join(bctx.GOROOT, "src", importPath)
	grNd := mx.VFS.Poke(grDir).Ls()
	if grNd.Some(pkgNdFilter) {
		return &PkgPath{Dir: grDir, ImportPath: importPath, Goroot: true}, nil
	}
	if goutil.ModEnabled(mx, srcDir) {
		return findPkgGm(mx, importPath, srcDir)
	}
	return findPkgGp(mx, bctx, importPath, srcDir)
}

func findPkgGp(mx *mg.Ctx, bctx *build.Context, importPath, srcDir string) (*PkgPath, error) {
	_, memo, err := mx.VFS.Memo(srcDir)
	if err != nil {
		return nil, err
	}
	type K struct {
		goutil.SrcDirKey
		importPath string
	}
	type V struct {
		p *PkgPath
		e error
	}
	k := K{goutil.MakeSrcDirKey(bctx, srcDir), importPath}
	v := memo.Read(k, func() interface{} {
		bpkg, err := bctx.Import(importPath, k.SrcDir, build.FindOnly)
		v := V{e: err}
		if err == nil {
			v.p = &PkgPath{
				Dir:        bpkg.Dir,
				ImportPath: bpkg.ImportPath,
				Goroot:     bpkg.Goroot,
			}
		}
		return v
	}).(V)
	return v.p, v.e
}

func findPkgGm(mx *mg.Ctx, importPath, srcDir string) (*PkgPath, error) {
	fileNd := goutil.ModFileNd(mx, srcDir)
	if fileNd == nil {
		return nil, os.ErrNotExist
	}
	// we depends on both go.mod and go.sum so we need to cache in the dir
	dirNd := fileNd.Parent()
	memo, _ := dirNd.Memo()
	type K struct {
		goutil.SrcDirKey
		importPath string
	}
	type V struct {
		p *PkgPath
		e error
	}
	bctx := goutil.BuildContext(mx)
	k := K{goutil.MakeSrcDirKey(bctx, srcDir), importPath}
	v := memo.Read(k, func() interface{} {
		mf, err := loadModSum(dirNd.Path())
		if err != nil {
			return V{e: err}
		}
		v := V{}
		v.p, v.e = mf.find(mx, bctx, importPath)
		return v
	}).(V)
	return v.p, v.e
}

type modFile struct {
	Dir  string
	Path string
	Deps map[string]module.Version
	File *modfile.File
}

type modVer struct {
	Path       string
	Version    string
	ImportPath string
	Suffix     string
}

func (mf *modFile) requireMV(importPath string) (_ module.Version, isSelf bool, found bool) {
	if mv := mf.File.Module.Mod; mv.Path == importPath {
		return mv, true, true
	}
	if mv, ok := mf.Deps[importPath]; ok {
		return mv, false, true
	}
	if p := mgutil.PathParent(importPath); p != "" {
		return mf.requireMV(p)
	}
	return module.Version{}, false, false
}

func (mf *modFile) require(importPath string) (_ modVer, isSelf bool, _ error) {
	v, isSelf, found := mf.requireMV(importPath)
	if !found {
		return modVer{}, false, fmt.Errorf("require(%s) not found in %s", importPath, mf.Path)
	}
	mv := modVer{
		ImportPath: importPath,
		Path:       v.Path,
		Version:    v.Version,
	}
	mv.Suffix = strings.TrimPrefix(mv.ImportPath, mv.Path)
	mv.Suffix = strings.TrimLeft(mv.Suffix, "/")
	if isSelf && mv.Suffix == "" {
		return modVer{}, false, fmt.Errorf("cannot import the main module `%s`", importPath)
	}
	return mv, isSelf, nil
}

// TODO: add support `std`. stdlib pkgs are vendored, so AFAIK, it's not used yet.
func (mf *modFile) find(mx *mg.Ctx, bctx *build.Context, importPath string) (*PkgPath, error) {
	mv, isSelf, err := mf.require(importPath)
	if err != nil {
		return nil, err
	}
	lsPkg := func(pfx, sfx string) *PkgPath {
		dir := filepath.Join(pfx, filepath.FromSlash(sfx))
		ok := mx.VFS.Poke(dir).Ls().Some(pkgNdFilter)
		if ok {
			return &PkgPath{Dir: dir, ImportPath: mv.ImportPath}
		}
		return nil
	}

	// if we importing a sub/module package don't search anywhere else
	if isSelf {
		dir := filepath.Dir(mf.Path)
		if p := lsPkg(dir, mv.Suffix); p != nil {
			return p, nil
		}
		return nil, fmt.Errorf("cannot find module src for `%s` in main module `%s`", mv.ImportPath, dir)
	}

	// check local vendor first to support un-imaginable use-cases like editing third-party packages.
	// we don't care about BS like `-mod=vendor`
	if p := lsPkg(filepath.Join(mf.Dir, "vendor"), mv.ImportPath); p != nil {
		return p, nil
	}

	// then check pkg/mod
	mpath, err := module.EncodePath(mv.Path)
	if err != nil {
		return nil, err
	}
	roots := map[string]bool{mx.VFS.Poke(bctx.GOROOT).Poke("src").Path(): true}
	gopath := mgutil.PathList(bctx.GOPATH)
	pkgMod := filepath.FromSlash("pkg/mod/" + mpath + "@" + mv.Version)
	for _, gp := range gopath {
		roots[mx.VFS.Poke(gp).Poke("src").Path()] = true
		if p := lsPkg(filepath.Join(gp, pkgMod), mv.Suffix); p != nil {
			return p, nil
		}
	}

	// then check all the parent vendor dirs. we've already checked mf.Dir
	for sd := mx.VFS.Poke(mf.Dir).Parent(); !sd.IsRoot(); sd = sd.Parent() {
		dir := sd.Path()
		if roots[dir] {
			break
		}
		if p := lsPkg(filepath.Join(dir, "vendor"), mv.ImportPath); p != nil {
			return p, nil
		}
	}

	// then check GOROOT to support the `std` module
	if p := lsPkg(filepath.Join(bctx.GOROOT, "src", "vendor"), mv.ImportPath); p != nil {
		return p, nil
	}

	return nil, fmt.Errorf("cannot find module src for `%s` using `%s`", importPath, mf.Path)
}

// TODO: add support for `replace`
func loadModSum(dir string) (*modFile, error) {
	gomod := filepath.Join(dir, "go.mod")
	modSrc, err := ioutil.ReadFile(gomod)
	if err != nil {
		return nil, err
	}
	mf := &modFile{
		Dir:  dir,
		Path: gomod,
		Deps: map[string]module.Version{},
	}
	mf.File, err = modfile.ParseLax(gomod, modSrc, nil)
	if err != nil {
		return nil, err
	}
	for _, r := range mf.File.Require {
		mf.Deps[r.Mod.Path] = r.Mod
	}
	gosum := filepath.Join(dir, "go.sum")
	sumSrc, err := ioutil.ReadFile(gosum)
	if err != nil {
		return mf, nil
	}
	for _, ln := range bytes.Split(sumSrc, []byte{'\n'}) {
		fields := bytes.Fields(ln)
		if len(fields) != 3 {
			continue
		}
		mv := module.Version{Path: string(fields[0]), Version: string(fields[1])}
		if !semver.IsValid(mv.Version) {
			continue
		}
		if _, exists := mf.Deps[mv.Path]; exists {
			continue
		}
		mf.Deps[mv.Path] = mv
	}
	return mf, nil
}
