package golang

import (
	"bytes"
	"fmt"
	"margo.sh/mg"
	"margo.sh/mgpf"
	"math"
	"regexp"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"
)

var (
	mgcDbg = struct {
		sync.RWMutex
		f func(format string, a ...interface{})
	}{}
)

func mgcDbgf(format string, a ...interface{}) {
	mgcDbg.RLock()
	defer mgcDbg.RUnlock()

	if f := mgcDbg.f; f != nil {
		if !strings.HasSuffix(format, "\n") {
			format += "\n"
		}
		f("margocode: "+format, a...)
	}
}

type MarGocodeCtl struct {
	mg.ReducerType

	// Whether or not to print debugging info related to the gocode cache
	// used by the Gocode and GocodeCalltips reducers
	Debug bool

	cmdMap map[string]func(*mg.CmdCtx)
}

func (mgc *MarGocodeCtl) ReducerInit(mx *mg.Ctx) {
	mgc.cmdMap = map[string]func(*mg.CmdCtx){
		"help": mgc.helpCmd,
		"cache-list-by-key": func(cx *mg.CmdCtx) {
			mgc.listCacheCmd(cx, func(ents []mgcCacheEnt, i, j int) bool {
				return ents[i].Key < ents[j].Key
			})
		},
		"cache-list-by-dur": func(cx *mg.CmdCtx) {
			mgc.listCacheCmd(cx, func(ents []mgcCacheEnt, i, j int) bool {
				return ents[i].Dur < ents[j].Dur
			})
		},
		"cache-prune": mgc.pruneCacheCmd,
	}

	if mgc.Debug {
		mgcDbg.Lock()
		mgcDbg.f = mx.Log.Dbg.Printf
		mgcDbg.Unlock()
	}
}

func (mgc *MarGocodeCtl) Reduce(mx *mg.Ctx) *mg.State {
	st := mx.State

	switch mx.Action.(type) {
	case mg.RunCmd:
		st = st.AddBuiltinCmds(mgc.cmds()...)
	}

	return st
}

func (mgc *MarGocodeCtl) cmds() mg.BuiltinCmdList {
	return mg.BuiltinCmdList{
		mg.BuiltinCmd{
			Name: "margocodectl",
			Desc: "introspect and manage the margocode cache and state",
			Run:  mgc.cmd,
		},
	}
}

func (mgc *MarGocodeCtl) listCacheCmd(cx *mg.CmdCtx, less func(ents []mgcCacheEnt, i, j int) bool) {
	defer cx.Output.Close()

	ents := mgcSharedCache.entries()
	if len(ents) == 0 {
		fmt.Fprintln(cx.Output, "The cache is empty")
		return
	}

	buf := &bytes.Buffer{}
	tbw := tabwriter.NewWriter(cx.Output, 1, 4, 1, ' ', 0)
	defer tbw.Flush()

	digits := int(math.Floor(math.Log10(float64(len(ents)))) + 1)
	sfxFormat := "\t%s\t%s\n"
	hdrFormat := "%s" + sfxFormat
	rowFormat := fmt.Sprintf("%%%dd/%d", digits, len(ents)) + sfxFormat

	sort.Slice(ents, func(i, j int) bool { return less(ents, i, j) })
	fmt.Fprintf(buf, hdrFormat, "Count:", "Package Key:", "Import Duration:")
	for i, e := range ents {
		fmt.Fprintf(buf, rowFormat, i+1, e.Key, mgpf.D(e.Dur))
	}
	tbw.Write(buf.Bytes())
}

func (mgc *MarGocodeCtl) pruneCacheCmd(cx *mg.CmdCtx) {
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

	ents := mgcSharedCache.prune(pats...)
	for _, e := range ents {
		fmt.Fprintln(cx.Output, "Pruned:", e.Key)
	}
	fmt.Fprintln(cx.Output, "Pruned", len(ents), "entries")
	debug.FreeOSMemory()
}

func (mgc *MarGocodeCtl) helpCmd(cx *mg.CmdCtx) {
	defer cx.Output.Close()

	cx.Output.Write([]byte(`Usage: margocodectl $subcmd [args...]
	cache-prune [regexp, or path...] - remove packages matching glob from the cache. default: '.*'
	cache-list-by-key                - list cached packages, sorted by key
	cache-list-by-dur                - list cached packages, sorted by import duration
`))
}

func (mgc *MarGocodeCtl) cmd(cx *mg.CmdCtx) *mg.State {
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
