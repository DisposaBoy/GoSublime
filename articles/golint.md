How to run custom linters in GoSublime.
=============================

Until https://github.com/DisposaBoy/GoSublime/issues/220 is closed, you can run your own linters e.g.`go build`, `go vet` or [golint](https://github.com/golang/lint) commands by using `comp-lint`, it's able to run user-commands when you save.

Please note that comp-lint may effectively disable the live linter that currently only does `syntax` and `flag` checks.

note: replace `ctrl` with `super` on OS X

To enable comp-lint, add the following settings to your user settings file ( `ctrl+dot`, `ctrl+5` ). These settings are further documented in the default settings file ( `ctrl+dot`,`ctrl+4` )


	// enable comp-lint, this will effectively disable the live linter
	"comp_lint_enabled": true,

	// list of commands to run
	"comp_lint_commands": [
		// run `golint` on all files in the package
		// "shell":true is required in order to run the command through your shell (to expand `*.go`)
		// also see: the documentation for the `shell` setting in the default settings file ctrl+dot,ctrl+4
		{"cmd": ["golint *.go"], "shell": true}
		
		// run go vet on the package
		// {"cmd": ["go", "vet"]}

		// run `go install` on the package. GOBIN is set,
		// so `main` packages shouldn't result in the installation of a binary
		// {"cmd": ["go", "install"]}
	],
	
	"on_save": [
		// run comp-lint when you save,
		// naturally, you can also bind this command `gs_comp_lint`
		// to a key binding if you want
		{"cmd": "gs_comp_lint"}
	]


