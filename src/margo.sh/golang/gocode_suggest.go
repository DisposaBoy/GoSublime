package golang

import (
	"github.com/mdempsky/gocode/suggest"
	"go/build"
	"go/importer"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/gcexportdata"
	"margo.sh/golang/internal/srcimporter"
	"margo.sh/mg"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

type gsuOpts struct {
	ProposeBuiltins bool
	Debug           bool
	Source          bool
}

type gsuImpRes struct {
	pkg *types.Package
	err error
}

type gcSuggest struct {
	gsuOpts
	sync.Mutex
	imp *gsuImporter
}

func newGcSuggest(mx *mg.Ctx, o gsuOpts) *gcSuggest {
	gsu := &gcSuggest{gsuOpts: o}
	gsu.imp = gsu.newGsuImporter(mx)
	return gsu
}

func (gsu *gcSuggest) newUnderlyingSrcImporter(mx *mg.Ctx, overlay types.ImporterFrom) types.ImporterFrom {
	return srcimporter.New(
		overlay,

		BuildContext(mx),
		token.NewFileSet(),
		map[string]*types.Package{},
	)
}

func (gsu *gcSuggest) newUnderlyingBinImporter(mx *mg.Ctx) types.ImporterFrom {
	if runtime.Compiler == "gc" {
		return gcexportdata.NewImporter(token.NewFileSet(), map[string]*types.Package{})
	}
	return importer.Default().(types.ImporterFrom)
}

func (gsu *gcSuggest) newUnderlyingImporter(mx *mg.Ctx, overlay types.ImporterFrom) types.ImporterFrom {
	// TODO: switch to source importer only
	if gsu.Source {
		return gsu.newUnderlyingSrcImporter(mx, overlay)
	}
	return gsu.newUnderlyingBinImporter(mx)
}

func (gsu *gcSuggest) newGsuImporter(mx *mg.Ctx) *gsuImporter {
	gi := &gsuImporter{
		mx:  mx,
		bld: BuildContext(mx),
		gsu: gsu,
		res: map[mgcCacheKey]gsuImpRes{},
	}
	return gi
}

func (gsu *gcSuggest) candidates(mx *mg.Ctx) []suggest.Candidate {
	defer mx.Profile.Push("candidates").Pop()
	gsu.Lock()
	defer gsu.Unlock()

	defer func() {
		if e := recover(); e != nil {
			mx.Log.Printf("gocode/suggest panic: %s\n%s\n", e, debug.Stack())
		}
	}()

	cfg := suggest.Config{
		// we no longer support contextual build env :(
		// GoSublime works around this for other packages by restarting the agent
		// if GOPATH changes, so we should be ok
		Importer:   gsu.imp,
		Builtin:    gsu.ProposeBuiltins,
		IgnoreCase: true,
	}
	if gsu.Debug {
		cfg.Logf = func(f string, a ...interface{}) {
			f = "Gocode: " + f
			if !strings.HasSuffix(f, "\n") {
				f += "\n"
			}
			mx.Log.Dbg.Printf(f, a...)
		}
	}

	v := mx.View
	src, _ := v.ReadAll()
	if len(src) == 0 {
		return nil
	}

	l, _ := cfg.Suggest(v.Filename(), src, v.Pos)
	return l
}

type gsuPkgInfo struct {
	// the import path
	Path string

	// the abs path to the package directory
	Dir string

	// the key used for caching
	Key mgcCacheKey

	// whether or not this is a stdlib package
	Std bool
}

type gsuImporter struct {
	mx  *mg.Ctx
	bld *build.Context
	gsu *gcSuggest
	res map[mgcCacheKey]gsuImpRes
}

func (gi *gsuImporter) Import(path string) (*types.Package, error) {
	return gi.ImportFrom(path, ".", 0)
}

func (gi *gsuImporter) ImportFrom(impPath, srcDir string, mode types.ImportMode) (impPkg *types.Package, err error) {
	// TODO: add mode to the key somehow?
	// mode is reserved, but currently not used so it's not a problem
	// but if it's used in the future, the importer result could depend on it
	//
	// adding it to the key might complicate the pkginfo api because it's called
	// by code that doesn't know anything about mode
	pkgInf, err := gi.pkgInfo(impPath, srcDir)
	if err != nil {
		mctl.dbgf("pkgInfo(%q, %q): %s\n", impPath, srcDir, err)
		return nil, err
	}

	// we cache the results of the underlying importer for this *session*
	// because if it fails, we could potentialy end up in a loop
	// trying to import the package again.
	if res, ok := gi.res[pkgInf.Key]; ok {
		return res.pkg, res.err
	}

	pkg, err := gi.importFrom(pkgInf, mode)
	res := gsuImpRes{pkg: pkg, err: err}
	gi.res[pkgInf.Key] = res

	return pkg, err
}

func (gi *gsuImporter) importFrom(pkgInf gsuPkgInfo, mode types.ImportMode) (impPkg *types.Package, err error) {
	mx, gsu := gi.mx, gi.gsu

	defer mx.Profile.Push("gsuImport: " + pkgInf.Path).Pop()

	// I think we need to use a new underlying importer every time
	// because they cache imports which might depend on srcDir
	//
	// they also have a fileset which could possibly grow indefinitely.
	// I assume using different filesets is ok since we don't make use of it directly
	//
	// at least for the srcImporter, we pass in our own importer as the overlay
	// so we should still get some caching
	//
	// binary imports should hopefully still be fast enough
	underlying := gsu.newUnderlyingImporter(mx, gi)
	if pkgInf.Std && pkgInf.Path == "unsafe" {
		return types.Unsafe, nil
	}

	if res, ok := gi.res[pkgInf.Key]; ok {
		return res.pkg, res.err
	}

	if e, ok := mctl.pkgs.get(pkgInf.Key); ok {
		return e.Pkg, nil
	}

	impStart := time.Now()
	typPkg, err := underlying.ImportFrom(pkgInf.Path, pkgInf.Dir, mode)
	impDur := time.Since(impStart)

	if err == nil {
		mctl.pkgs.put(mgcCacheEnt{Key: pkgInf.Key, Pkg: typPkg, Dur: impDur})
	} else {
		mctl.dbgf("%T.ImportFrom(%q, %q): %s\n", underlying, pkgInf.Path, pkgInf.Dir, err)
	}

	return typPkg, err
}

func (gi *gsuImporter) pkgInfo(impPath, srcDir string) (gsuPkgInfo, error) {
	// TODO: support cache these ops?
	// at least on the session level, the importFrom cache should cover this
	//
	// TODO: support go modules
	// at this time, go/packages appears to be extremely slow
	// it takes 100ms+ just to load the errors packages in LoadFiles mode

	bpkg, err := gi.bld.Import(impPath, srcDir, build.FindOnly)
	if err != nil {
		return gsuPkgInfo{}, err
	}
	return gsuPkgInfo{
		Path: bpkg.ImportPath,
		Dir:  bpkg.Dir,
		Key:  mkMgcCacheKey(gi.gsu.Source, bpkg.Dir),
		Std:  bpkg.Goroot,
	}, nil
}

func (gi *gsuImporter) pruneCacheOnReduce(mx *mg.Ctx) {
	switch mx.Action.(type) {
	case mg.ViewModified, mg.ViewSaved:
		// ViewSaved is probably not required, but saving might result in a `go install`
		// which results in an updated package.a file

		if pkgInf, err := gi.pkgInfo(".", mx.View.Dir()); err == nil {
			mctl.pkgs.del(pkgInf.Key)
		}
	}
}
