package margo

import (
	"margo.sh/golang"
	"margo.sh/mg"
	"time"
)

// Margo is the entry-point to margo
func Margo(m mg.Args) {
	// See the documentation for `mg.Reducer`
	// comments beginning with `gs:` denote features that replace old GoSublime settings

	// add our reducers (margo plugins) to the store
	// they are run in the specified order
	// and should ideally not block for more than a couple milliseconds
	m.Use(
		// MOTD keeps you updated about new versions and important announcements
		//
		// It adds a new command `motd.sync` available via the UserCmd palette as `Sync MOTD`
		//
		// Interval can be set in order to enable automatic update fetching.
		//
		// When new updates are found, it displays the message in the status bar
		// e.g. `★ margo.sh/cl/18.09.14 ★` a url where you see the upcoming changes before updating
		//
		// It sends the following data to the url https://api.margo.sh/motd.json:
		// * current editor plugin name e.g. `?client=gosublime`
		//   this tells us which editor plugin's changelog to check
		// * current editor plugin version e.g. `?tag=r18.09.14-1`
		//   this allows us to determine if there any updates
		// * whether or not this is the first request of the day e.g. `?firstHit=1`
		//   this allows us to get an estimated count of active users without storing
		//   any personally identifiable data
		//
		// No other data is sent. For more info contact privacy at kuroku.io
		//
		&mg.MOTD{
			// Interval, if set, specifies how often to automatically fetch messages from Endpoint
			// Interval: 3600e9, // automatically fetch updates every hour
		},

		mg.NewReducer(func(mx *mg.Ctx) *mg.State {
			// By default, events (e.g. ViewSaved) are triggered in all files.
			// Replace `mg.AllLangs` with `mg.Go` to restrict events to Go(-lang) files.
			// Please note, however, that this mode is not tested
			// and saving a non-go file will not trigger linters, etc. for that go pkg
			return mx.SetConfig(mx.Config.EnabledForLangs(
				mg.AllLangs,
			))
		}),

		// Add `go` command integration
		// this adds a new commands:
		// gs: these commands are all callable through 9o:
		// * go: Wrapper around the go command, adding linter support
		// * go.play: Automatically build and run go commands or run go test for packages
		//   with support for linting and unsaved files
		// * go.replay: Wrapper around go.play limited to a single instance
		//   by default this command is bound to ctrl+.,ctrl+r or cmd+.,cmd+r
		//
		// UserCmds are also added for `Go Play` and `Go RePlay`
		&golang.GoCmd{},

		// add the day and time to the status bar
		&DayTimeStatus{},

		// both GoFmt and GoImports will automatically disable the GoSublime version
		// you will need to install the `goimports` tool manually
		// https://godoc.org/golang.org/x/tools/cmd/goimports
		//
		// gs: this replaces settings `fmt_enabled`, `fmt_tab_indent`, `fmt_tab_width`, `fmt_cmd`
		//
		// golang.GoFmt,
		// or
		// golang.GoImports,

		// Configure general auto-completion behaviour
		&golang.MarGocodeCtl{
			// whether or not to include Test*, Benchmark* and Example* functions in the auto-completion list
			// gs: this replaces the `autocomplete_tests` setting
			ProposeTests: false,

			// Don't try to automatically import packages when auto-compeltion fails
			// e.g. when `json.` is typed, if auto-complete fails
			// "encoding/json" is imported and auto-complete attempted on that package instead
			// See AddUnimportedPackages
			NoUnimportedPackages: false,

			// If a package was imported internally for use in auto-completion,
			// insert it in the source code
			// See NoUnimportedPackages
			// e.g. after `json.` is typed, `import "encoding/json"` added to the code
			AddUnimportedPackages: false,

			// Don't preload packages to speed up auto-completion, etc.
			NoPreloading: false,

			// Don't suggest builtin types and functions
			// gs: this replaces the `autocomplete_builtins` setting
			NoBuiltins: false,
		},

		// Enable auto-completion
		// gs: this replaces the `gscomplete_enabled` setting
		&golang.Gocode{
			// show the function parameters. this can take up a lot of space
			ShowFuncParams: true,
		},

		// show func arguments/calltips in the status bar
		// gs: this replaces the `calltips` setting
		&golang.GocodeCalltips{},

		// use guru for goto-definition
		// new commands `goto.definition` and `guru.definition` are defined
		// gs: by default `goto.definition` is bound to ctrl+.,ctrl+g or cmd+.,cmd+g
		&golang.Guru{},

		// add some default context aware-ish snippets
		// gs: this replaces the `autocomplete_snippets` and `default_snippets` settings
		golang.Snippets,

		// add our own snippets
		// gs: this replaces the `snippets` setting
		MySnippets,

		// check the file for syntax errors
		// gs: this and other linters e.g. below,
		//     replaces the settings `gslint_enabled`, `lint_filter`, `comp_lint_enabled`,
		//     `comp_lint_commands`, `gslint_timeout`, `lint_enabled`, `linters`
		&golang.SyntaxCheck{},

		// Add user commands for running tests and benchmarks
		// gs: this adds support for the tests command palette `ctrl+.`,`ctrl+t` or `cmd+.`,`cmd+t`
		&golang.TestCmds{
			// additional args to add to the command when running tests and examples
			TestArgs: []string{},

			// additional args to add to the command when running benchmarks
			BenchArgs: []string{"-benchmem"},
		},

		// run `go install -i` on save
		// golang.GoInstall("-i"),
		// or
		// golang.GoInstallDiscardBinaries("-i"),
		//
		// GoInstallDiscardBinaries will additionally set $GOBIN
		// to a temp directory so binaries are not installed into your $GOPATH/bin
		//
		// the -i flag is used to install imported packages as well
		// it's only supported in go1.10 or newer

		// run `go vet` on save. go vet is ran automatically as part of `go test` in go1.10
		// golang.GoVet(),

		// run `go test -race` on save
		// golang.GoTest("-race"),

		// run `golint` on save
		// &golang.Linter{Name: "golint", Label: "Go/Lint"},

		// run gometalinter on save
		// &golang.Linter{Name: "gometalinter", Args: []string{
		// 	"--disable=gas",
		// 	"--fast",
		// }},
	)
}

// DayTimeStatus adds the current day and time to the status bar
type DayTimeStatus struct {
	mg.ReducerType
}

func (dts DayTimeStatus) RMount(mx *mg.Ctx) {
	// kick off the ticker when we start
	dispatch := mx.Store.Dispatch
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		for range ticker.C {
			dispatch(mg.Render)
		}
	}()
}

func (dts DayTimeStatus) Reduce(mx *mg.Ctx) *mg.State {
	// we always want to render the time
	// otherwise it will sometimes disappear from the status bar
	now := time.Now()
	format := "Mon, 15:04"
	if now.Second()%2 == 0 {
		format = "Mon, 15 04"
	}
	return mx.AddStatus(now.Format(format))
}

// MySnippets is a slice of functions returning our own snippets
var MySnippets = golang.SnippetFuncs(
	func(cx *golang.CompletionCtx) []mg.Completion {
		// if we're not in a block (i.e. function), do nothing
		if !cx.Scope.Is(golang.BlockScope) {
			return nil
		}

		return []mg.Completion{
			{
				Query: "if err",
				Title: "err != nil { return }",
				Src:   "if ${1:err} != nil {\n\treturn $0\n}",
			},
		}
	},
)
