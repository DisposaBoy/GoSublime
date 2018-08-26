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
)

var (
	gsuSharedCache = &gsuPkgCache{m: map[gsuImpKey]*types.Package{}}
)

// gsuImpKey is the key used for caching package imports
// it's the abs path of the package directory
type gsuImpKey string

type gsuOpts struct {
	ProposeBuiltins bool
	Debug           bool
	Source          bool
}

type gcSuggest struct {
	gsuOpts
	sync.Mutex
	imp *gsuImporter
}

func newGcSuggest(mx *mg.Ctx, o gsuOpts) *gcSuggest {
	gsu := &gcSuggest{gsuOpts: o}
	gsu.init(mx)
	return gsu
}

func (gsu *gcSuggest) init(mx *mg.Ctx) {
	gsu.imp = gsu.newGsuImporter(mx)
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
			gsu.dbgf(mx, f, a...)
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

func (gsu *gcSuggest) dbgf(mx *mg.Ctx, f string, a ...interface{}) {
	if !gsu.Debug {
		return
	}

	f = "Gocode: " + f
	if !strings.HasSuffix(f, "\n") {
		f += "\n"
	}

	mx.Log.Dbg.Printf(f, a...)
}

type gsuPkgInfo struct {
	// the import path
	Path string

	// the abs path to the package directory
	Dir string

	// the key used for caching
	Key gsuImpKey

	// whether or not this is a stdlib package
	Std bool
}

type gsuPkgCache struct {
	mu sync.RWMutex
	m  map[gsuImpKey]*types.Package
}

func (gp *gsuPkgCache) get(dbgf func(f string, a ...interface{}), k gsuImpKey) *types.Package {
	gp.mu.RLock()
	defer gp.mu.RUnlock()
	return gp.m[k]
}

func (gp *gsuPkgCache) put(dbgf func(f string, a ...interface{}), k gsuImpKey, p *types.Package) {
	if !p.Complete() {
		dbgf("gsuCache.put: not storing %s, it's incomplete\n", k)
		return
	}

	gp.mu.Lock()
	defer gp.mu.Unlock()

	gp.m[k] = p
	dbgf("gsuCache.put: %s\n", k)
}

func (gp *gsuPkgCache) del(dbgf func(f string, a ...interface{}), k gsuImpKey) {
	gp.mu.Lock()
	defer gp.mu.Unlock()

	if _, exists := gp.m[k]; !exists {
		return
	}

	delete(gp.m, k)
	dbgf("gsuCache.del: %s\n", k)
}

type gsuImporter struct {
	mx  *mg.Ctx
	bld *build.Context
	gsu *gcSuggest
}

func (gi *gsuImporter) dbgf(f string, a ...interface{}) {
	gi.gsu.dbgf(gi.mx, f, a...)
}

func (gi *gsuImporter) Import(path string) (*types.Package, error) {
	return gi.ImportFrom(path, ".", 0)
}

func (gi *gsuImporter) ImportFrom(impPath, srcDir string, mode types.ImportMode) (*types.Package, error) {
	mx, gsu := gi.mx, gi.gsu
	pkgs := gsuSharedCache

	defer mx.Profile.Push("gsuImport: " + impPath).Pop()

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

	pkgInf, err := gi.pkgInfo(impPath, srcDir)
	if err != nil {
		gsu.dbgf(mx, "build.Import(%q, %q): %s\n", impPath, srcDir, err)
		return nil, err
	}

	if pkgInf.Std && pkgInf.Path == "unsafe" {
		return types.Unsafe, nil
	}

	if p := pkgs.get(gi.dbgf, pkgInf.Key); p != nil {
		return p, nil
	}

	typPkg, err := underlying.ImportFrom(impPath, srcDir, mode)
	if err == nil {
		pkgs.put(gi.dbgf, pkgInf.Key, typPkg)
	} else {
		gi.dbgf("%T.ImportFrom(%q, %q): %s\n", underlying, impPath, srcDir, err)
	}
	return typPkg, err
}

func (gi *gsuImporter) pkgInfo(impPath, srcDir string) (gsuPkgInfo, error) {
	// TODO: support cache these ops?
	// it might not be worth the added complexity since we will get a lot of impPath=io
	// with a different srcPath which means we have to look it up anyway.
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
		Key:  gsuImpKey(bpkg.Dir),
		Std:  bpkg.Goroot,
	}, nil
}

func (gi *gsuImporter) pruneCacheOnReduce(mx *mg.Ctx) {
	switch mx.Action.(type) {
	case mg.ViewModified, mg.ViewSaved:
		// ViewSaved is probably not required, but saving might result in a `go install`
		// which results in an updated package.a file

		if pkgInf, err := gi.pkgInfo(".", mx.View.Dir()); err == nil {
			gsuSharedCache.del(gi.dbgf, pkgInf.Key)
		}
	}
}
