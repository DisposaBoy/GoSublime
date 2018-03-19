package margo

import (
	"disposa.blue/margo/golang"
	"disposa.blue/margo/mg"
	"time"
)

// Margo is the entry-point to margo
func Margo(ma mg.Args) {
	// add our reducers (margo plugins) to the store
	// they are run in the specified order
	// and should ideally not block for more than a couple milliseconds
	ma.Store.Use(
		// add the day and time to the status bar
		// DayTimeStatus,

		// both GoFmt and GoImports will automatically disable the GoSublime version
		// you will need to install the `goimports` tool manually
		// https://godoc.org/golang.org/x/tools/cmd/goimports
		//
		// golang.GoFmt,
		// or
		// golang.GoImports,

		// use gocode for autocompletion
		&golang.Gocode{
			// automatically install missing packages
			// Autobuild: true,

			// autocompete packages that are not yet imported
			// this goes well with GoImports
			UnimportedPackages: true,

			// show the function parameters. this can take up a lot of space
			ShowFuncParams: true,
		},

		// add some default context aware-ish snippets
		golang.Snippets,

		// add our own snippets

		// check the file for syntax errors
		&golang.SyntaxCheck{},

		// add our own snippets
		MySnippets,

		// run `go install` on save
		// or use GoInstallDiscardBinaries which will additionally set $GOBIN
		// to a temp directory so binaries are not installed into your $PATH
		//
		// golang.GoInstall(),
		// or
		// golang.GoInstallDiscardBinaries(),

		// run `go vet` on save. go vet is ran automatically as part of `go test` in go1.10
		// golang.GoVet(),

		// run `go test -race` on save
		// in go1.10, go vet is ran automatically
		golang.GoTest("-race"),

		// run `golint` on save
		// &golang.Linter{Name: "golint", Label: "Go/Lint"},
	)
}

// DayTimeStatus adds the current day and time to the status bar
var DayTimeStatus = mg.Reduce(func(mx *mg.Ctx) *mg.State {
	if _, ok := mx.Action.(mg.Started); ok {
		dispatch := mx.Store.Dispatch
		// kick off the ticker when we start
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			for range ticker.C {
				dispatch(mg.Render)
			}
		}()
	}

	// we always want to render the time
	// otherwise it will sometimes disappear from the status bar
	now := time.Now()
	format := "Mon, 15:04"
	if now.Second()%2 == 0 {
		format = "Mon, 15 04"
	}
	return mx.AddStatus(now.Format(format))
})

// MySnippets is a slice of functions returning our own snippets
var MySnippets = golang.SnippetFuncs{
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
}
