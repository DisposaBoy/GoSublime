package golang

import (
	"errors"
	"github.com/mdempsky/gocode/suggest"
	"go/build"
	"go/types"
	"margo.sh/mg"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

var (
	errImportCycleDetected = errors.New("import cycle detected")
)

type gsuOpts struct {
	ProposeBuiltins bool
	Debug           bool
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

func (gsu *gcSuggest) newGsuImporter(mx *mg.Ctx) *gsuImporter {
	gi := &gsuImporter{
		mx:  mx,
		bld: BuildContext(mx),
		gsu: gsu,
	}
	gi.res.m = map[mgcCacheKey]gsuImpRes{}
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

	// whether or not this is a stdlib package
	Std bool
}

func (p gsuPkgInfo) cacheKey(source bool) mgcCacheKey {
	return mgcCacheKey{gsuPkgInfo: p, Source: source}
}

type gsuImporter struct {
	mx  *mg.Ctx
	bld *build.Context
	gsu *gcSuggest

	res struct {
		sync.Mutex
		m map[mgcCacheKey]gsuImpRes
	}
}

func (gi *gsuImporter) Import(path string) (*types.Package, error) {
	return gi.ImportFrom(path, ".", 0)
}

func (gi *gsuImporter) ImportFrom(impPath, srcDir string, mode types.ImportMode) (pkg *types.Package, err error) {
	// TODO: add mode to the key somehow?
	// mode is reserved, but currently not used so it's not a problem
	// but if it's used in the future, the importer result could depend on it
	//
	// adding it to the key might complicate the pkginfo api because it's called
	// by code that doesn't know anything about mode
	pkgInf, err := mctl.pkgInfo(gi.mx, impPath, srcDir)
	if err != nil {
		mctl.dbgf("pkgInfo(%q, %q): %s\n", impPath, srcDir, err)
		return nil, err
	}
	newDefImpr, newFbkImpr, srcMode := mctl.importerFactories()
	k := pkgInf.cacheKey(srcMode)

	gi.res.Lock()
	res, seen := gi.res.m[k]
	if !seen {
		gi.res.m[k] = gsuImpRes{err: errImportCycleDetected}
	}
	gi.res.Unlock()

	// we cache the results of the underlying importer for this *session*
	// because if it fails, or there's an import cycle, we could potentialy end up in a loop
	// trying to import the package again.
	if seen {
		return res.pkg, res.err
	}
	defer func() {
		gi.res.Lock()
		defer gi.res.Unlock()

		gi.res.m[k] = gsuImpRes{pkg: pkg, err: err}
	}()

	defImpr := newDefImpr(gi.mx, gi)
	pkg, err = gi.importFrom(defImpr, k, mode)
	complete := err == nil && pkg.Complete()
	if complete {
		return pkg, nil
	}

	mctl.dbgf("importFrom(%q, %q): default=%T: complete=%v, err=%v\n",
		k.Path, k.Dir, defImpr, complete, err,
	)

	// no fallback allowed
	if newFbkImpr == nil {
		return pkg, err
	}

	// problem1:
	// if the pkg import fails we will offer no completion
	//
	// problem 2:
	// if it succeeds, but is incomplete we offer completion with `invalid-type` failures
	// i.e. completion stops working at random points for no obvious reason
	//
	// assumption:
	//   it's better to risk using stale data (bin imports)
	//   as opposed to offering no completion at all
	//
	// risks:
	// we will end up caching the result, but that shouldn't be a big deal
	// because if the pkg is edited, thus (possibly) making it importable,
	// we will remove it from the cache anyway.
	// there is the issue about mixing binary (potentially incomplete) pkgs with src pkgs
	// but we were already not going to return anything, so it *shouldn't* apply here

	fbkImpr := newFbkImpr(gi.mx, gi)
	fbkPkg, fbkErr := gi.importFrom(fbkImpr, k.fallback(), mode)
	fbkComplete := fbkErr == nil && fbkPkg.Complete()
	switch {
	case fbkComplete:
		pkg, err = fbkPkg, nil
	case fbkPkg != nil && pkg == nil:
		pkg, err = fbkPkg, fbkErr
	}

	mctl.dbgf("importFrom(%q, %q): fallback=%T: complete=%v, err=%v\n",
		k.Path, k.Dir, fbkImpr, fbkComplete, fbkErr,
	)

	return pkg, err
}

func (gi *gsuImporter) importFrom(underlying types.ImporterFrom, k mgcCacheKey, mode types.ImportMode) (*types.Package, error) {
	defer gi.mx.Profile.Push("gsuImport: " + k.Path).Pop()

	if k.Std && k.Path == "unsafe" {
		return types.Unsafe, nil
	}

	if e, ok := mctl.pkgs.get(k); ok {
		return e.Pkg, nil
	}

	impStart := time.Now()
	pkg, err := underlying.ImportFrom(k.Path, k.Dir, mode)
	impDur := time.Since(impStart)

	if err == nil {
		mctl.pkgs.put(mgcCacheEnt{Key: k, Pkg: pkg, Dur: impDur})
	} else {
		mctl.dbgf("%T.ImportFrom(%q, %q): %s\n", underlying, k.Path, k.Dir, err)
	}

	return pkg, err
}
