package golang

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/gcexportdata"
	"log"
	"margo.sh/golang/internal/srcimporter"
	"margo.sh/mg"
	"margo.sh/mgpf"
	"margo.sh/mgutil"
	"math"
	"os/exec"
	"path"
	"regexp"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
)

const (
	// SrcImporterWithFallback tells the importer use source code, then fall-back to a binary package
	SrcImporterWithFallback ImporterMode = iota

	// SrcImporterOnly tells the importer use source only, with no fall-back
	SrcImporterOnly

	// BinImporterOnly tells the importer use binary packages only, with no fall-back
	BinImporterOnly
)

var (
	mctl *marGocodeCtl
)

func init() {
	mctl = newMarGocodeCtl()
	mg.DefaultReducers.Before(mctl)
}

type importerFactory func(mx *mg.Ctx, overlay types.ImporterFrom) types.ImporterFrom

// ImporterMode specifies the mode in which the corresponding importer should operate
type ImporterMode int

type marGocodeCtl struct {
	mg.ReducerType

	mxQ *mgutil.ChanQ

	mu     sync.RWMutex
	mgcctl MarGocodeCtl
	pkgs   *mgcCache
	cmdMap map[string]func(*mg.CmdCtx)
	logs   *log.Logger

	ipbn struct {
		sync.RWMutex
		m map[string]string
	}
}

func (mgc *marGocodeCtl) importerFactories() (newDefaultImporter, newFallbackImporter importerFactory, srcMode bool) {
	s := mgc.newSrcImporter
	b := mgc.newBinImporter
	switch mgc.cfg().ImporterMode {
	case SrcImporterWithFallback:
		return s, b, true
	case SrcImporterOnly:
		return s, nil, true
	case BinImporterOnly:
		return b, nil, false
	default:
		panic("unreachable")
	}
}

// importPathByName returns an import path whose pkg's name is pkgName
func (mgc *marGocodeCtl) importPathByName(pkgName string) string {
	// try the cache first
	// it includes packages the user actually imported
	// so there's theoretically a better chance of importing the ideal package
	// in cases where there's a name collision
	if p := mgc.ipbnFromCache(pkgName); p != "" {
		return p
	}

	mgc.ipbn.RLock()
	defer mgc.ipbn.RUnlock()
	return mgc.ipbn.m[pkgName]
}

func (mgc *marGocodeCtl) ipbnFromCache(pkgName string) string {
	mgc.mu.RLock()
	defer mgc.mu.RUnlock()

	importPath := ""
	mgc.pkgs.forEach(func(e mgcCacheEnt) bool {
		if p := e.Pkg; p.Name() == pkgName {
			importPath = e.Key.Path
			return false
		}
		return true
	})
	return importPath
}

// newSrcImporter returns a new instance a source code importer
func (mgc *marGocodeCtl) newSrcImporter(mx *mg.Ctx, overlay types.ImporterFrom) types.ImporterFrom {
	return srcimporter.New(
		overlay,

		BuildContext(mx),
		token.NewFileSet(),
		map[string]*types.Package{},
	)
}

// newBinImporter returns a new instance of a binary package importer for packages compiled by runtime.Compiler
func (mgc *marGocodeCtl) newBinImporter(mx *mg.Ctx, overlay types.ImporterFrom) types.ImporterFrom {
	if runtime.Compiler == "gc" {
		return gcexportdata.NewImporter(token.NewFileSet(), map[string]*types.Package{})
	}
	return importer.Default().(types.ImporterFrom)
}

func (mgc *marGocodeCtl) processQ(mx *mg.Ctx) {
	defer func() { recover() }()

	switch mx.Action.(type) {
	case mg.ViewModified, mg.ViewSaved:
		mgc.autoPruneCache(mx)
	case mg.ViewActivated:
		mgc.preloadPackages(mx)
	}
}

func (mgc *marGocodeCtl) preloadPackages(mx *mg.Ctx) {
	if mgc.cfg().NoPreloading {
		return
	}

	v := mx.View
	src, _ := v.ReadAll()
	if len(src) == 0 {
		return
	}

	fset := token.NewFileSet()
	af, _ := parser.ParseFile(fset, v.Filename(), src, parser.ImportsOnly)
	if af == nil || len(af.Imports) == 0 {
		return
	}

	dir := v.Dir()
	gsu := mgc.newGcSuggest(mx)
	for _, spec := range af.Imports {
		gsu.imp.ImportFrom(unquote(spec.Path.Value), dir, 0)
	}
}

func (mgc *marGocodeCtl) autoPruneCache(mx *mg.Ctx) {
	// TODO: do this in a goroutine?
	// we're not directly in the QueryCompletions hot path
	// but we *are* in a subscriber, so we're blocking the store
	// if something like pkginfo or DebugPrune is slow,
	// we will end up blocking the next reduction

	pkgInf, err := mgc.pkgInfo(mx, ".", mx.View.Dir())
	if err == nil {
		for _, source := range []bool{true, false} {
			mgc.pkgs.del(pkgInf.cacheKey(source))
		}
	}

	dpr := mgc.cfg().DebugPrune

	if dpr != nil {
		mgc.pkgs.forEach(func(e mgcCacheEnt) bool {
			if dpr(e.Pkg) {
				mgc.pkgs.del(e.Key)
			}
			return true
		})
	}
}

func (mgc *marGocodeCtl) cfg() MarGocodeCtl {
	mgc.mu.RLock()
	defer mgc.mu.RUnlock()

	return mgc.mgcctl
}

func (mgc *marGocodeCtl) configure(f func(*marGocodeCtl)) {
	mgc.mu.Lock()
	defer mgc.mu.Unlock()

	f(mgc)
}

func newMarGocodeCtl() *marGocodeCtl {
	mgc := &marGocodeCtl{}
	mgc.pkgs = &mgcCache{m: map[mgcCacheKey]mgcCacheEnt{}}
	mgc.cmdMap = map[string]func(*mg.CmdCtx){
		"help":                mgc.helpCmd,
		"cache-list":          mgc.cacheListCmd,
		"cache-prune":         mgc.cachePruneCmd,
		"unimported-packages": mgc.unimportedPackagesCmd,
	}
	mgc.mxQ = mgutil.NewChanQ(10)
	go func() {
		for v := range mgc.mxQ.C() {
			mgc.processQ(v.(*mg.Ctx))
		}
	}()
	return mgc
}

func (mgc *marGocodeCtl) RCond(mx *mg.Ctx) bool {
	if mx.LangIs(mg.Go) {
		return true
	}
	if act, ok := mx.Action.(mg.RunCmd); ok {
		for _, c := range mgc.cmds() {
			if c.Name == act.Name {
				return true
			}
		}
	}
	return false
}

func (mgc *marGocodeCtl) RMount(mx *mg.Ctx) {
	go mgc.initIPBN(mx)
}

func (mgc *marGocodeCtl) Reduce(mx *mg.Ctx) *mg.State {
	switch mx.Action.(type) {
	case mg.RunCmd:
		return mx.AddBuiltinCmds(mgc.cmds()...)
	case mg.ViewModified, mg.ViewSaved, mg.ViewActivated:
		// ViewSaved is probably not required, but saving might result in a `go install`
		// which results in an updated package.a file
		mgc.mxQ.Put(mx)
	}

	return mx.State
}

func (mgc *marGocodeCtl) initIPBN(mx *mg.Ctx) {
	// TODO: scan GOPATH as well

	cmd := exec.Command("go", "list", "-f={{.Name}},{{.ImportPath}}", "std")
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd.Stdout = outBuf
	cmd.Stderr = errBuf
	if err := cmd.Run(); err != nil {
		mx.Log.Printf("``` %s ```\n%s\n%s\n", mgutil.QuoteCmd(cmd.Path, cmd.Args...), errBuf.Bytes(), err)
	}

	mgc.ipbn.Lock()
	defer mgc.ipbn.Unlock()

	if mgc.ipbn.m == nil {
		mgc.ipbn.m = map[string]string{}
	}
	m := mgc.ipbn.m

	skip := map[string]bool{
		"":         true,
		"cmd":      true,
		"internal": true,
		"vendor":   true,
		"main":     true,
	}

	scanner := bufio.NewScanner(outBuf)
	for scanner.Scan() {
		line := strings.SplitN(scanner.Text(), ",", 2)
		if len(line) != 2 {
			continue
		}
		nm, pth := line[0], line[1]
		if nm == "" {
			nm = path.Base(pth)
		}
		if !skip[nm] && !skip[strings.Split(pth, "/")[0]] {
			m[nm] = pth
		}
	}
}

// srcMode returns true if the importMode is not SrcImporterOnly or SrcImporterWithFallback
func (mgc *marGocodeCtl) srcMode() bool {
	switch mgc.cfg().ImporterMode {
	case SrcImporterOnly, SrcImporterWithFallback:
		return true
	case BinImporterOnly:
		return false
	default:
		panic("unreachable")
	}
}

func (mgc *marGocodeCtl) pkgInfo(mx *mg.Ctx, impPath, srcDir string) (gsuPkgInfo, error) {
	// TODO: cache these ops?
	// it might not be worth the added complexity since we will get a lot of impPath=io
	// with a different srcPath which means we have to look it up anyway.
	//
	// TODO: support go modules
	// at this time, go/packages appears to be extremely slow
	// it takes 100ms+ just to load the errors packages in LoadFiles mode
	//
	// in eiter case, in go1.11 we might end up calling `go list` which is very slow

	bpkg, err := BuildContext(mx).Import(impPath, srcDir, build.FindOnly)
	if err != nil {
		return gsuPkgInfo{}, err
	}
	return gsuPkgInfo{
		Path: bpkg.ImportPath,
		Dir:  bpkg.Dir,
		Std:  bpkg.Goroot,
	}, nil
}

func (mgc *marGocodeCtl) dbgf(format string, a ...interface{}) {
	mgc.mu.Lock()
	logs := mgc.logs
	mgc.mu.Unlock()

	if logs == nil {
		return
	}

	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}
	logs.Printf("margocode: "+format, a...)
}

func (mgc *marGocodeCtl) cmds() mg.BuiltinCmdList {
	return mg.BuiltinCmdList{
		mg.BuiltinCmd{
			Name: "margocodectl",
			Desc: "introspect and manage the margocode cache and state",
			Run:  mgc.cmd,
		},
	}
}

func (mgc *marGocodeCtl) cacheListCmd(cx *mg.CmdCtx) {
	defer cx.Output.Close()

	ents := mgc.pkgs.entries()
	lessFuncs := map[string]func(i, j int) bool{
		"path": func(i, j int) bool {
			return ents[i].Key.Path < ents[j].Key.Path
		},
		"dur": func(i, j int) bool {
			return ents[i].Dur < ents[j].Dur
		},
	}
	orderNames := func() string {
		l := make([]string, 0, len(lessFuncs))
		for k, _ := range lessFuncs {
			l = append(l, k)
		}
		sort.Strings(l)
		return strings.Join(l, "|")
	}()

	by := "path"
	desc := false
	flags := flag.NewFlagSet(cx.Name, flag.ContinueOnError)
	flags.SetOutput(cx.Output)
	flags.BoolVar(&desc, "desc", desc, "Order results in descending order")
	flags.StringVar(&by, "by", by, "Field to order by: "+orderNames)
	err := flags.Parse(cx.Args)
	if err != nil {
		return
	}
	less, ok := lessFuncs[by]
	if !ok {
		fmt.Fprintf(cx.Output, "Unknown order=%s. Expected one of: %s\n", by, orderNames)
		flags.Usage()
		return
	}
	if desc {
		lf := less
		less = func(i, j int) bool { return lf(j, i) }
	}

	if len(ents) == 0 {
		fmt.Fprintln(cx.Output, "The cache is empty")
		return
	}

	buf := &bytes.Buffer{}
	tbw := tabwriter.NewWriter(cx.Output, 1, 4, 1, ' ', 0)
	defer tbw.Flush()

	digits := int(math.Floor(math.Log10(float64(len(ents)))) + 1)
	sfxFormat := "\t%s\t%s\t%s\n"
	hdrFormat := "%s" + sfxFormat
	rowFormat := fmt.Sprintf("%%%dd/%d", digits, len(ents)) + sfxFormat

	sort.Slice(ents, less)
	fmt.Fprintf(buf, hdrFormat, "Count:", "Path:", "Duration:", "Mode:")
	for i, e := range ents {
		mode := "bin"
		if e.Key.Source {
			mode = "src"
		}
		fmt.Fprintf(buf, rowFormat, i+1, e.Key.Path, mgpf.D(e.Dur), mode)
	}
	tbw.Write(buf.Bytes())
}

func (mgc *marGocodeCtl) unimportedPackagesCmd(cx *mg.CmdCtx) {
	defer cx.Output.Close()

	type ent struct {
		nm  string
		pth string
	}

	mgc.ipbn.RLock()
	ents := make([]ent, 0, len(mgc.ipbn.m))
	for nm, pth := range mgc.ipbn.m {
		ents = append(ents, ent{nm: nm, pth: pth})
	}
	mgc.ipbn.RUnlock()

	buf := &bytes.Buffer{}
	tbw := tabwriter.NewWriter(cx.Output, 1, 4, 1, ' ', 0)
	defer tbw.Flush()

	digits := int(math.Floor(math.Log10(float64(len(ents)))) + 1)
	sfxFormat := "\t%s\t%s\n"
	hdrFormat := "%s" + sfxFormat
	rowFormat := fmt.Sprintf("%%%dd/%d", digits, len(ents)) + sfxFormat

	sort.Slice(ents, func(i, j int) bool {
		return ents[i].nm < ents[j].nm
	})

	fmt.Fprintf(buf, hdrFormat, "Count:", "Name:", "ImportPath:")
	for i, e := range ents {
		fmt.Fprintf(buf, rowFormat, i+1, e.nm, e.pth)
	}
	tbw.Write(buf.Bytes())
}

func (mgc *marGocodeCtl) cachePruneCmd(cx *mg.CmdCtx) {
	defer cx.Output.Close()

	args := cx.Args
	if len(args) == 0 {
		args = []string{".*"}
	}

	pats := make([]*regexp.Regexp, 0, len(args))
	for _, s := range args {
		p, err := regexp.Compile(s)
		if err == nil {
			pats = append(pats, p)
		} else {
			fmt.Fprintf(cx.Output, "Error: regexp.Compile(%s): %s\n", s, err)
		}
	}

	ents := mgc.pkgs.prune(pats...)
	for _, e := range ents {
		fmt.Fprintln(cx.Output, "Pruned:", e.Key)
	}
	fmt.Fprintln(cx.Output, "Pruned", len(ents), "entries")
	debug.FreeOSMemory()
}

func (mgc *marGocodeCtl) helpCmd(cx *mg.CmdCtx) {
	defer cx.Output.Close()

	cx.Output.Write([]byte(`Usage: ` + cx.Name + ` $subcmd [args...]
	cache-prune [regexp, or path...] - remove packages matching glob from the cache. default: '.*'
	cache-list                       - list cached packages, see '` + cx.Name + ` cache-list --help' for more details
	unimported-packages              - list the packages for the UnimportedPackages feature
`))
}

func (mgc *marGocodeCtl) newGcSuggest(mx *mg.Ctx) *gcSuggest {
	mgc.mu.RLock()
	defer mgc.mu.RUnlock()

	gsu := &gcSuggest{cfg: mgc.cfg()}
	gsu.imp = gsu.newGsuImporter(mx)
	return gsu
}

func (mgc *marGocodeCtl) cmd(cx *mg.CmdCtx) *mg.State {
	cmd := mgc.helpCmd
	if len(cx.Args) > 0 {
		sub := cx.Args[0]
		if c, ok := mgc.cmdMap[sub]; ok {
			cmd = c
			cx = cx.Copy(func(cx *mg.CmdCtx) {
				cx.Args = cx.Args[1:]
			})
		} else {
			fmt.Fprintln(cx.Output, "Unknown subcommand:", sub)
		}
	}
	go cmd(cx)
	return cx.State
}

type MarGocodeCtl struct {
	mg.ReducerType

	// Whether or not to print debugging info related to the gocode cache
	// used by the Gocode and GocodeCalltips reducers
	Debug bool

	// DebugPrune returns true if pkg should be removed from the cache
	DebugPrune func(pkg *types.Package) bool

	// The mode in which the types.Importer shouler operate
	// By default it is SrcImporterWithFallback
	ImporterMode ImporterMode

	// Don't try to automatically import packages when auto-compeltion fails
	// e.g. when `json.` is typed, if auto-complete fails
	// "encoding/json" is imported and auto-complete attempted on that package instead
	// See AddUnimportedPackages
	NoUnimportedPackages bool

	// If a package was imported internally for use in auto-completion,
	// insert it in the source code
	// See NoUnimportedPackages
	// e.g. after `json.` is typed, `import "encoding/json"` added to the code
	AddUnimportedPackages bool

	// Don't preload packages to speed up auto-completion, etc.
	NoPreloading bool

	// Don't propose builtin types and functions
	NoBuiltins bool

	// Whether or not to propose builtin types and functions
	ProposeTests bool
}

func (mgc *MarGocodeCtl) RInit(mx *mg.Ctx) {
	mctl.configure(func(m *marGocodeCtl) {
		m.mgcctl = *mgc
		if mgc.Debug {
			m.logs = mx.Log.Dbg
		}
	})
}

func (mgc *MarGocodeCtl) Reduce(mx *mg.Ctx) *mg.State {
	return mx.State
}
