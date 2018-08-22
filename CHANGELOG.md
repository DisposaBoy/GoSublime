


# Donate:

If you find GoSublime useful and would like to support me and future development of GoSublime,
please donate via one of the available methods on https://margo.kuroku.io/donate?_r=gs




# Changes:


## 18.08.22

* merge all shell env vars named ^(MARGO|GO|CGO)\w+ into the GoSublime environment
  this ensures new env vars like GOPROXY and GO111MODULE work correctly

* try to prevent `GO111MODULE` leaking into the agent build process

* add support for UserCmd prompts

	this enables the creation of UserCmds like the following, without dedicated support from margo:

	mg.UserCmd{
		Title:   "GoRename",
		Name:    "gorename",
		Args:    []string{"-offset={{.View.Filename}}:#{{.View.Pos}}", "-to={{index .Prompts 0}}"},
		Prompts: []string{"New Name"},
	}

* fix #853 a build failure when using snap packaged go1.10

* fix caching of packages in GOPATH when doing gocode completion
  this *might* slow completion, but there should no longer be any stale non-GOROOT package completions

* add new `Source` option to use source code for gocode completions

	*this will most likely be very slow*

		&golang.Gocode{ Source: true }
		&golang.GocodeCalltips{ Source: true }



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



