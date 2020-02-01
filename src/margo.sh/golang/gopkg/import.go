package gopkg

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/rogpeppe/go-internal/modfile"
	"github.com/rogpeppe/go-internal/module"
	"github.com/rogpeppe/go-internal/semver"
	"go/build"
	"margo.sh/golang/goutil"
	"margo.sh/mg"
	"margo.sh/mgutil"
	"margo.sh/vfs"
	"os"
	"path/filepath"
	"strings"
)

var (
	pkgModFilepath     = string(filepath.Separator) + "pkg" + string(filepath.Separator) + "mod" + string(filepath.Separator)
	errPkgPathNotFound = errors.New("pkg path not found")
)

func ScanFilter(de *vfs.Dirent) bool {
	nm := de.Name()
	if nm[0] == '.' || nm[0] == '_' || nm == "testdata" || nm == "node_modules" {
		return false
	}
	return de.IsDir() || strings.HasSuffix(nm, ".go")
}

func ImportDir(mx *mg.Ctx, dir string) (*Pkg, error) {
	if !filepath.IsAbs(dir) {
		return nil, fmt.Errorf("ImportDir: %s is not an absolute path", dir)
	}
	return ImportDirNd(mx, mx.VFS.Poke(dir))
}

func ImportDirNd(mx *mg.Ctx, dir *vfs.Node) (*Pkg, error) {
	return importDirNd(mx, dir, true)
}

func importDirNd(mx *mg.Ctx, nd *vfs.Node, poke bool) (*Pkg, error) {
	var cl *vfs.NodeList
	if poke {
		cl = nd.Ls()
	} else {
		cl = nd.Children()
	}
	ls := cl.Filter(pkgNdFilter).Nodes()
	if len(ls) == 0 {
		if poke {
			return nil, &build.NoGoError{Dir: nd.Path()}
		}
		return nil, nil
	}
	bctx := goutil.BuildContext(mx)
	type K struct{ GOROOT, GOPATH string }
	type V struct {
		p *Pkg
		e error
	}
	k := K{GOROOT: bctx.GOROOT, GOPATH: bctx.GOPATH}
	if !poke {
		v, _ := nd.PeekMemo(k).(V)
		return v.p, v.e
	}
	v := nd.ReadMemo(k, func() interface{} {
		p, err := importDir(mx, nd, bctx, ls)
		return V{p: p, e: err}
	}).(V)
	return v.p, v.e
}

func PeekDir(mx *mg.Ctx, dir string) *Pkg {
	return PeekDirNd(mx, mx.VFS.Peek(dir))
}

func PeekDirNd(mx *mg.Ctx, dir *vfs.Node) *Pkg {
	p, _ := importDirNd(mx, dir, false)
	return p
}

func pkgNdFilter(nd *vfs.Node) bool {
	nm := nd.Name()
	return nm[0] != '.' && nm[0] != '_' &&
		strings.HasSuffix(nm, ".go") &&
		// there's no such thing as a ~~killer videotape~~go package with only test files
		!strings.HasSuffix(nm, "_test.go")
}

func importDir(mx *mg.Ctx, nd *vfs.Node, bctx *build.Context, ls []*vfs.Node) (*Pkg, error) {
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
		return findPkgGm(mx, importPath, srcDir, nil)
	}
	if p, err := findPkgPm(mx, importPath, srcDir); err == nil {
		return p, nil
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

func findPkgPm(mx *mg.Ctx, importPath, srcDir string) (*PkgPath, error) {
	srcDir = filepath.Clean(srcDir)
	pmPos := strings.Index(srcDir, pkgModFilepath)
	if pmPos < 0 {
		return nil, errPkgPathNotFound
	}
	vPos := strings.Index(srcDir[pmPos:], "@v")
	if vPos < 0 {
		return nil, errPkgPathNotFound
	}
	vPos += pmPos
	modDir := srcDir
	if i := strings.IndexByte(srcDir[vPos:], filepath.Separator); i >= 0 {
		modDir = srcDir[:vPos+i]
	}
	mod := filepath.ToSlash(modDir[pmPos+len(pkgModFilepath) : vPos])
	sfx := strings.TrimPrefix(importPath, mod)
	if sfx != "" && sfx[0] != '/' {
		return nil, errPkgPathNotFound
	}
	ver := modDir[vPos+1:]
	if !semver.IsValid(ver) {
		return nil, errPkgPathNotFound
	}
	dir := filepath.Join(modDir, filepath.ToSlash(sfx))
	if !mx.VFS.Poke(dir).Ls().Some(pkgNdFilter) {
		return nil, errPkgPathNotFound
	}
	return &PkgPath{
		Dir:        dir,
		ImportPath: importPath,
	}, nil
}

func findPkgGm(mx *mg.Ctx, importPath, srcDir string, mp *ModPath) (*PkgPath, error) {
	fileNd := goutil.ModFileNd(mx, srcDir)
	if fileNd == nil {
		return nil, os.ErrNotExist
	}
	dir := fileNd.Parent().Path()
	if mp != nil && mp.Dir == dir {
		return mp.FindPkg(mx, importPath, srcDir)
	}
	return (&ModPath{Parent: mp, Dir: dir}).FindPkg(mx, importPath, srcDir)
}

type ModPath struct {
	Parent *ModPath
	Dir    string
}

func (mp *ModPath) FindPkg(mx *mg.Ctx, importPath, srcDir string) (*PkgPath, error) {
	if mp == nil {
		return FindPkg(mx, importPath, srcDir)
	}
	for ; mp != nil; mp = mp.Parent {
		if p, err := mp.findPkg(mx, importPath, srcDir); err == nil {
			return p, nil
		}
	}
	if p, err := findPkgPm(mx, importPath, srcDir); err == nil {
		return p, nil
	}
	return findPkgGp(mx, goutil.BuildContext(mx), importPath, srcDir)
}

func (mp *ModPath) findPkg(mx *mg.Ctx, importPath, srcDir string) (*PkgPath, error) {
	dirNd := mx.VFS.Poke(mp.Dir)
	bctx := goutil.BuildContext(mx)
	mf, err := loadModSumNd(mx, dirNd)
	if err != nil {
		return nil, err
	}
	return mf.find(mx, bctx, importPath, mp)
}

type modFile struct {
	Dir  string
	Path string
	Deps map[string]modDep
	File *modfile.File
}

type modDep struct {
	Dir     string
	ModPath string
	SubPkg  string
	Version string

	oldPath string
}

func (mf *modFile) requireMD(modPath string) (_ modDep, found bool) {
	if md, ok := mf.Deps[modPath]; ok {
		return md, true
	}
	if p := mgutil.PathParent(modPath); p != "" {
		return mf.requireMD(p)
	}
	return modDep{}, false
}

func (mf *modFile) require(importPath string) (modDep, error) {
	md, found := mf.requireMD(importPath)
	if !found {
		return modDep{}, fmt.Errorf("require(%s) not found in %s", importPath, mf.Path)
	}
	modPath := md.ModPath
	if md.oldPath != "" {
		modPath = md.oldPath
	}
	md.SubPkg = strings.TrimPrefix(importPath, modPath)
	md.SubPkg = strings.TrimLeft(md.SubPkg, "/")
	return md, nil
}

// TODO: support `std`. stdlib pkgs are vendored, so AFAIK, it's not used yet.
func (mf *modFile) find(mx *mg.Ctx, bctx *build.Context, importPath string, mp *ModPath) (pp *PkgPath, err error) {
	defer func() {
		if pp != nil {
			pp.Mod = &ModPath{Dir: mf.Dir, Parent: mp}
		}
	}()

	md, err := mf.require(importPath)
	if err != nil {
		return nil, err
	}
	lsPkg := func(pfx, sfx string) *PkgPath {
		dir := filepath.Join(pfx, filepath.FromSlash(sfx))
		ok := mx.VFS.Poke(dir).Ls().Some(pkgNdFilter)
		if ok {
			return &PkgPath{Dir: dir, ImportPath: importPath}
		}
		return nil
	}

	// if we're importing a self/sub-module or local replacement package don't search anywhere else
	if md.Dir != "" {
		if p := lsPkg(md.Dir, md.SubPkg); p != nil {
			return p, nil
		}
		return nil, fmt.Errorf("cannot find local/replacement package `%s` in `%s`", importPath, md.Dir)
	}

	// local vendor first to support un-imaginable use-cases like editing third-party packages.
	// we don't care about BS like `-mod=vendor`
	searchLocalVendor := func() *PkgPath {
		return lsPkg(filepath.Join(mf.Dir, "vendor"), importPath)
	}
	mpath, err := module.EncodePath(md.ModPath)
	if err != nil {
		return nil, err
	}
	grSrc := mx.VFS.Poke(bctx.GOROOT).Poke("src")
	roots := map[string]bool{grSrc.Path(): true}
	searchPkgMod := func() *PkgPath {
		gopath := mgutil.PathList(bctx.GOPATH)
		pkgMod := filepath.FromSlash("pkg/mod/" + mpath + "@" + md.Version)
		for _, gp := range gopath {
			roots[mx.VFS.Poke(gp).Poke("src").Path()] = true
			if p := lsPkg(filepath.Join(gp, pkgMod), md.SubPkg); p != nil {
				return p
			}
		}
		return nil
	}
	// check all the parent vendor dirs. we check mf.Dir separately
	searchOtherVendors := func() *PkgPath {
		for sd := mx.VFS.Poke(mf.Dir).Parent(); !sd.IsRoot(); sd = sd.Parent() {
			dir := sd.Path()
			if roots[dir] {
				break
			}
			if p := lsPkg(filepath.Join(dir, "vendor"), importPath); p != nil {
				return p
			}
		}
		return nil
	}
	// check GOROOT/vendor to support the `std` module
	searchGrVendor := func() *PkgPath {
		return lsPkg(filepath.Join(bctx.GOROOT, "src", "vendor"), importPath)
	}
	search := []func() *PkgPath{
		searchLocalVendor,
		searchPkgMod,
		searchOtherVendors,
		searchGrVendor,
	}
	if !strings.Contains(strings.SplitN(importPath, "/", 2)[0], ".") {
		// apparently import paths without dots are reserved for the stdlib
		// checking first also avoids the many misses for each stdlib pkg
		search = []func() *PkgPath{
			searchGrVendor,
			searchLocalVendor,
			searchPkgMod,
			searchOtherVendors,
		}
	}
	for _, f := range search {
		if p := f(); p != nil {
			return p, nil
		}
	}
	if md.oldPath != "" {
		return nil, fmt.Errorf("cannot find `%s` replacement `%s` using `%s`", importPath, md.ModPath, mf.Path)
	}
	return nil, fmt.Errorf("cannot find `%s` using `%s`", importPath, mf.Path)
}

func loadModSumNd(mx *mg.Ctx, dirNd *vfs.Node) (*modFile, error) {
	type K struct{}
	type V struct {
		mf *modFile
		e  error
	}
	v := dirNd.ReadMemo(K{}, func() interface{} {
		v := V{}
		v.mf, v.e = loadModSum(mx, dirNd.Path())
		return v
	}).(V)
	return v.mf, v.e
}

func loadModSum(mx *mg.Ctx, dir string) (*modFile, error) {
	gomod := filepath.Join(dir, "go.mod")
	modSrc, err := mx.VFS.ReadBlob(gomod).ReadFile()
	if err != nil {
		return nil, err
	}
	mf := &modFile{
		Dir:  dir,
		Path: gomod,
		Deps: map[string]modDep{},
	}
	mf.File, err = modfile.Parse(gomod, modSrc, nil)
	if err != nil {
		return nil, err
	}

	for _, r := range mf.File.Require {
		mf.Deps[r.Mod.Path] = modDep{
			ModPath: r.Mod.Path,
			Version: r.Mod.Version,
		}
	}

	for _, r := range mf.File.Replace {
		md := modDep{
			oldPath: r.Old.Path,
			ModPath: r.New.Path,
			Version: r.New.Version,
		}
		if dir := r.New.Path; modfile.IsDirectoryPath(dir) {
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(mf.Dir, dir)
			}
			nd := mx.VFS.Poke(dir)
			// replacement isn't valid unless the go.mod file exists
			// TODO: should we ignore this rule? I don't know what problem it solves
			// but it makes is more annoying to just point a module at a local directory
			if nd.Poke("go.mod").IsFile() {
				md.Dir = nd.Path()
				// the path is a filesystem path, not an import path
				md.ModPath = r.Old.Path
			}
		}
		mf.Deps[r.Old.Path] = md
	}

	self := mf.File.Module.Mod
	mf.Deps[self.Path] = modDep{
		Dir:     mf.Dir,
		ModPath: self.Path,
		Version: self.Version,
	}

	gosum := filepath.Join(dir, "go.sum")
	sumSrc, err := mx.VFS.ReadBlob(gosum).ReadFile()
	if err != nil {
		return mf, nil
	}
	for _, ln := range bytes.Split(sumSrc, []byte{'\n'}) {
		fields := bytes.Fields(ln)
		if len(fields) != 3 {
			continue
		}
		md := modDep{ModPath: string(fields[0]), Version: string(fields[1])}
		if !semver.IsValid(md.Version) {
			continue
		}
		if _, exists := mf.Deps[md.ModPath]; exists {
			continue
		}
		mf.Deps[md.ModPath] = md
	}
	return mf, nil
}
