


# Donate:

If you find GoSublime useful and would like to support me and future development of GoSublime,
please donate via one of the available methods on https://margo.kuroku.io/donate?_r=gs




# Changes:


## 18.08.15

* fix missing `go` command integration by default

* you may need to add the reducer `&golang.GoCmd{}`

*	this adds new commands (callable through 9o):

	* `go`: Wrapper around the go command, adding linter support

	* `go.play`: Automatically build and run go commands or run go test for packages
	 with support for linting and unsaved files

	* `go.replay`: Wrapper around go.play limited to a single instance
	 by default this command is bound to `ctrl+.,ctrl+r` or `cmd+.,cmd+r`

	UserCmds (`ctrl+.,ctrl+c` / `cmd+.,cmd+c`) are also added for `Go Play` and `Go RePlay`



