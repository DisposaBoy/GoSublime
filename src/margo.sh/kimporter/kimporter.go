package kimporter

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"go/build"
	"go/token"
	"go/types"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/net/context"
	"golang.org/x/sync/errgroup"
	"golang.org/x/tools/go/gcexportdata"
	"margo.sh/golang/gopkg"
	"margo.sh/golang/goutil"
	"margo.sh/mg"
	"margo.sh/mgutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

var (
	sharedCache = &stateCache{m: map[stateKey]*state{}}
	pkgC        = func() *types.Package {
		p := types.NewPackage("C", "C")
		p.MarkComplete()
		return p
	}()
)

type stateKey struct {
	ImportPath   string
	Dir          string
	CheckFuncs   bool
	CheckImports bool
	Tests        bool
	Tags         string
	GOARCH       string
	GOOS         string
	GOROOT       string
	GOPATH       string
	SrcMapHash   string
}

type stateCache struct {
	mu sync.Mutex
	m  map[stateKey]*state
}

func (sc *stateCache) state(mx *mg.Ctx, k stateKey) *state {
	// TODO: support vfs invalidation.
	// we can't (currently) make use of .Memo because it deletes the data
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if v, ok := sc.m[k]; ok {
		return v
	}
	v := &state{stateKey: k}
	sc.m[k] = v
	return v
}

type state struct {
	stateKey

	mu      sync.Mutex
	err     error
	pkg     *types.Package
	checked bool
}

func (ks *state) reset() {
	ks.pkg = nil
	ks.err = nil
	ks.checked = false
}

func (ks *state) result() (*types.Package, error) {
	switch {
	case !ks.checked:
		return nil, fmt.Errorf("import cycle via %s", ks.ImportPath)
	case ks.err != nil:
		return ks.pkg, ks.err
	case !ks.pkg.Complete():
		// Package exists but is not complete - we cannot handle this
		// at the moment since the source importer replaces the package
		// wholesale rather than augmenting it (see #19337 for details).
		// Return incomplete package with error (see #16088).
		return ks.pkg, fmt.Errorf("reimported partially imported package %q", ks.ImportPath)
	default:
		return ks.pkg, nil
	}
}

type Config struct {
	SrcMap        map[string][]byte
	CheckFuncs    bool
	CheckImports  bool
	NoConcurrency bool
	Tests         bool
}

type Importer struct {
	cfg  Config
	mx   *mg.Ctx
	bld  *build.Context
	ks   *state
	mp   *gopkg.ModPath
	par  *Importer
	tags string
	hash string
}

func (kp *Importer) Import(path string) (*types.Package, error) {
	return kp.ImportFrom(path, ".", 0)
}

func (kp *Importer) ImportFrom(ipath, srcDir string, mode types.ImportMode) (*types.Package, error) {
	// TODO: add support for unsaved-files without a package
	if mode != 0 {
		panic("non-zero import mode")
	}
	if ipath == "C" {
		return pkgC, nil
	}
	if ipath == "unsafe" {
		return types.Unsafe, nil
	}
	if p, err := filepath.Abs(srcDir); err == nil {
		srcDir = p
	}
	if !filepath.IsAbs(srcDir) {
		return nil, fmt.Errorf("srcDir is not absolute: %s", srcDir)
	}
	pp, err := kp.findPkg(ipath, srcDir)
	if err != nil {
		return nil, err
	}
	return kp.importPkg(pp)
}

func (kp *Importer) findPkg(ipath, srcDir string) (*gopkg.PkgPath, error) {
	return kp.mp.FindPkg(kp.mx, ipath, srcDir)
}

func (kp *Importer) stateKey(pp *gopkg.PkgPath) stateKey {
	cfg := kp.cfg
	return stateKey{
		ImportPath:   pp.ImportPath,
		Dir:          pp.Dir,
		CheckFuncs:   cfg.CheckFuncs,
		CheckImports: cfg.CheckImports,
		Tests:        cfg.Tests,
		Tags:         kp.tags,
		GOOS:         kp.bld.GOOS,
		GOARCH:       kp.bld.GOARCH,
		GOROOT:       kp.bld.GOROOT,
		SrcMapHash:   kp.hash,
		GOPATH:       strings.Join(mgutil.PathList(kp.bld.GOPATH), string(filepath.ListSeparator)),
	}
}

func (kp *Importer) state(pp *gopkg.PkgPath) *state {
	return sharedCache.state(kp.mx, kp.stateKey(pp))
}

func (kp *Importer) detectCycle(ks *state) error {
	l := []string{ks.ImportPath}
	for p := kp.par; p != nil; p = p.par {
		if p.ks == nil {
			continue
		}
		if p.ks.ImportPath != "" {
			l = append(l, p.ks.ImportPath)
		}
		if p.ks.Dir == ks.Dir {
			return fmt.Errorf("import cycle: %s", strings.Join(l, " <~ "))
		}
	}
	return nil
}

func (kp *Importer) importPkg(pp *gopkg.PkgPath) (*types.Package, error) {
	title := "Kim-Porter: import(" + pp.Dir + ")"
	defer kp.mx.Profile.Push(title).Pop()
	defer kp.mx.Begin(mg.Task{Title: title}).Done()

	ks := kp.state(pp)
	kx := kp.branch(ks, pp)
	if err := kx.detectCycle(ks); err != nil {
		return nil, err
	}
	ks.mu.Lock()
	defer ks.mu.Unlock()

	if ks.checked {
		return ks.result()
	}
	ks.reset()
	ks.checked = true
	ks.pkg, ks.err = kx.check(ks, pp)
	return ks.result()
}

func (kp *Importer) check(ks *state, pp *gopkg.PkgPath) (*types.Package, error) {
	fset := token.NewFileSet()
	bp, _, astFiles, err := parseDir(kp.mx, kp.bld, fset, pp.Dir, kp.cfg.SrcMap, ks)
	if err != nil {
		return nil, err
	}

	imports, err := kp.loadImports(ks, bp)
	if err != nil {
		return nil, err
	}

	if len(bp.CgoFiles) != 0 {
		pkg, err := kp.importCgoPkg(pp, imports)
		if err == nil {
			return pkg, nil
		}
	}

	var hardErr error
	tc := types.Config{
		FakeImportC:              true,
		IgnoreFuncBodies:         !ks.CheckFuncs,
		DisableUnusedImportCheck: !ks.CheckImports,
		Error: func(err error) {
			if te, ok := err.(types.Error); ok && !te.Soft && hardErr == nil {
				hardErr = err
			}
		},
		Importer: kp,
		Sizes:    types.SizesFor(kp.bld.Compiler, kp.bld.GOARCH),
	}
	pkg, err := tc.Check(bp.ImportPath, fset, astFiles, nil)
	if err == nil && hardErr != nil {
		err = hardErr
	}
	return pkg, err
}

func (kp *Importer) importCgoPkg(pp *gopkg.PkgPath, imports map[string]*types.Package) (*types.Package, error) {
	name := `go`
	args := []string{`list`, `-e`, `-export`, `-f={{.Export}}`, pp.Dir}
	ctx, cancel := context.WithCancel(context.Background())
	title := mgutil.QuoteCmd(name, args...)
	defer kp.mx.Profile.Push(title).Pop()
	defer kp.mx.Begin(mg.Task{Title: title, Cancel: cancel}).Done()

	buf := &bytes.Buffer{}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = pp.Dir
	cmd.Stdout = buf
	cmd.Env = kp.mx.Env.Environ()
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s: %s", title, err)
	}
	fn := string(bytes.TrimSpace(buf.Bytes()))
	f, err := os.Open(fn)
	if err != nil {
		return nil, fmt.Errorf("cannot open %s.a: %s", pp.ImportPath, err)
	}
	defer f.Close()
	rd, err := gcexportdata.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("cannot create export data reader for %s from %s: %s", pp.ImportPath, fn, err)
	}
	pkg, err := gcexportdata.Read(rd, token.NewFileSet(), imports, pp.ImportPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read export data for %s from %s: %s", pp.ImportPath, fn, err)
	}
	return pkg, nil
}

func (kp *Importer) loadImports(ks *state, bp *build.Package) (map[string]*types.Package, error) {
	paths := mgutil.StrSet(bp.Imports)
	if ks.Tests {
		paths = paths.Add(bp.TestImports...)
	}
	imports := make(map[string]*types.Package, len(paths))
	mu := sync.Mutex{}
	doImport := func(ipath string) error {
		pkg, err := kp.ImportFrom(ipath, bp.Dir, 0)
		if err == nil {
			mu.Lock()
			imports[ipath] = pkg
			mu.Unlock()
		}
		return err
	}
	if kp.cfg.NoConcurrency || len(paths) < 2 {
		for _, ipath := range paths {
			if err := doImport(ipath); err != nil {
				return imports, err
			}
		}
		return imports, nil
	}
	imps := make(chan string, len(paths))
	for _, ipath := range paths {
		imps <- ipath
	}
	close(imps)
	errg := &errgroup.Group{}
	for i := 0; i < mgutil.MinNumCPU(len(paths)); i++ {
		errg.Go(func() error {
			for ipath := range imps {
				if err := doImport(ipath); err != nil {
					return err
				}
			}
			return nil

		})
	}
	return imports, errg.Wait()
}

func (kp *Importer) setupJs(pp *gopkg.PkgPath) {
	fs := kp.mx.VFS
	nd := fs.Poke(kp.bld.GOROOT).Poke("src/syscall/js")
	if fs.Poke(pp.Dir) != nd && fs.Poke(kp.mx.View.Dir()) != nd {
		return
	}
	bld := *kp.bld
	bld.GOOS = "js"
	bld.GOARCH = "wasm"
	kp.bld = &bld
}

func (kp *Importer) branch(ks *state, pp *gopkg.PkgPath) *Importer {
	kx := *kp
	if pp.Mod != nil {
		kx.mp = pp.Mod
	}
	// user settings don't apply when checking deps
	kx.cfg.CheckFuncs = false
	kx.cfg.CheckImports = false
	kx.cfg.Tests = false
	kx.ks = ks
	kx.par = kp
	kx.setupJs(pp)
	return &kx
}

func New(mx *mg.Ctx, cfg *Config) *Importer {
	bld := goutil.BuildContext(mx)
	kp := &Importer{
		mx:   mx,
		bld:  bld,
		tags: tagsStr(bld.BuildTags),
	}
	if cfg != nil {
		kp.cfg = *cfg
		kp.hash = srcMapHash(cfg.SrcMap)
	}
	return kp
}

func srcMapHash(m map[string][]byte) string {
	if len(m) == 0 {
		return ""
	}
	fns := make(sort.StringSlice, len(m))
	for fn, _ := range m {
		fns = append(fns, fn)
	}
	fns.Sort()
	b2, _ := blake2b.New256(nil)
	for _, fn := range fns {
		b2.Write([]byte(fn))
		b2.Write(m[fn])
	}
	return hex.EncodeToString(b2.Sum(nil))
}

func tagsStr(l []string) string {
	switch len(l) {
	case 0:
		return ""
	case 1:
		return l[0]
	}
	s := append(sort.StringSlice{}, l...)
	s.Sort()
	return strings.Join(s, " ")
}
