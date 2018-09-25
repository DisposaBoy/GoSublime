package golang

import (
	"bytes"
	"flag"
	"fmt"
	"go/build"
	"go/importer"
	"go/token"
	"go/types"
	"golang.org/x/tools/go/gcexportdata"
	"log"
	"margo.sh/golang/internal/srcimporter"
	"margo.sh/mg"
	"margo.sh/mgpf"
	"margo.sh/mgutil"
	"math"
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
	pkgs   *mgcCache
	cmdMap map[string]func(*mg.CmdCtx)
	logs   *log.Logger
	dbgPrn func(*types.Package) bool
	mode   ImporterMode
}

func (mgc *marGocodeCtl) importerFactories() (newDefaultImporter, newFallbackImporter importerFactory, srcMode bool) {
	mgc.mu.RLock()
	defer mgc.mu.RUnlock()

	s := mgc.newSrcImporter
	b := mgc.newBinImporter
	switch mgc.mode {
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

	mgc.mu.RLock()
	dpr := mgc.dbgPrn
	mgc.mu.RUnlock()

	if dpr != nil {
		mgc.pkgs.forEach(func(e mgcCacheEnt) {
			if dpr(e.Pkg) {
				mgc.pkgs.del(e.Key)
			}
		})
	}
}

func (mgc *marGocodeCtl) conf(mx *mg.Ctx, ctl *MarGocodeCtl) {
	mgc.mu.Lock()
	defer mgc.mu.Unlock()

	if ctl.Debug {
		mgc.logs = mx.Log.Dbg
	}
	mgc.dbgPrn = ctl.DebugPrune
	mgc.mode = ctl.ImporterMode
}

func newMarGocodeCtl() *marGocodeCtl {
	mgc := &marGocodeCtl{}
	mgc.pkgs = &mgcCache{m: map[mgcCacheKey]mgcCacheEnt{}}
	mgc.cmdMap = map[string]func(*mg.CmdCtx){
		"help":        mgc.helpCmd,
		"cache-list":  mgc.cacheListCmd,
		"cache-prune": mgc.cachePruneCmd,
	}
	mgc.mxQ = mgutil.NewChanQ(10)
	go func() {
		for v := range mgc.mxQ.C() {
			mgc.autoPruneCache(v.(*mg.Ctx))
		}
	}()
	return mgc
}

func (mgc *marGocodeCtl) Reduce(mx *mg.Ctx) *mg.State {
	switch mx.Action.(type) {
	case mg.RunCmd:
		return mx.AddBuiltinCmds(mgc.cmds()...)
	case mg.ViewModified, mg.ViewSaved:
		// ViewSaved is probably not required, but saving might result in a `go install`
		// which results in an updated package.a file
		mgc.mxQ.Put(mx)
	}

	return mx.State
}

// srcMode returns true if the importMode is not SrcImporterOnly or SrcImporterWithFallback
func (mgc *marGocodeCtl) srcMode() bool {
	mgc.mu.RLock()
	defer mgc.mu.RUnlock()

	switch mgc.mode {
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
`))
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
}

func (mgc *MarGocodeCtl) ReducerInit(mx *mg.Ctx) {
	mctl.conf(mx, mgc)
}

func (mgc *MarGocodeCtl) Reduce(mx *mg.Ctx) *mg.State {
	return mx.State
}
