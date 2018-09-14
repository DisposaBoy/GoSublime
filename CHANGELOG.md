


# Donate:

If you find GoSublime useful and would like to support me and future development of GoSublime,
please donate via one of the available methods on https://margo.kuroku.io/donate?_r=gs




# Changes:


## 18.09.14

Note: a future release will set `Source: true` for golang.Gocode and golang.GocodeCalltips.
You're encouraged to switch it on manually if you haven't done so already.
This makes gocode use the source code instead of pre-compiled packages.
It's more accurate and up-to-date but might be slightly slower and use more memory.
If you find any new issues after switching, please file issues at margo.sh/gs/i


* This release adds a new experimental update notifier.

  MOTD keeps you updated about new versions and important announcements

  It adds a new command `motd.sync` available via the UserCmd palette as `Sync MOTD`

  `Interval` can be set in order to enable automatic update fetching.

  When new updates are found, it displays the message in the status bar
  e.g. `★ margo.sh/cl/18.09.14 ★` a url where you see the upcoming changes before updating

  It sends the following data to the url https://api.margo.sh/motd.json:
  * current editor plugin name e.g. `?client=gosublime`
    this tells us which editor plugin's changelog to check
  * current editor plugin version e.g. `?tag=r18.09.14-1`
    this allows us to determine if there any updates
  * whether or not this is the first request of the day e.g. `?firstHit=1`
    this allows us to get an estimated count of active users without storing
    any personally identifiable data

  No other data is sent. For more info contact privacy at kuroku.io

  To enabled it, add the following reducer:

    &mg.MOTD{
      // Interval, if set, specifies how often to automatically fetch messages from Endpoint
      // Interval: 3600e9, // automatically fetch updates every hour
    },

  You will need to restart Sublime Text.
  Unless you uncomment/set `Interval`, you will need to manually check for updates
  using the `Sync MOTD` command from the usercmd palette
  `ctrl+.`,`ctrl+c` / `super+.`,`super+c`


* The `GoSulime: Go` syntax was switched to a new syntax based on the Go syntax shipped in Sublime Text
  * if you find any breakages, please file an issue at margo.sh/gs/i
  * if you prefer the colouring of the previous version, you can switch back to the old syntax
    via `Menu > View > Syntax > Open all with current extension as... > GoSublime > GoSublime: Go (Deprecated)`
    please not that this version is buggy and will not receive any fixes


* golang.Gocode, golang.GocodeCalltips:
  * reduce memory use with `Source: true`
  * support syscall/js


* golang.Guru gained support for syscall/js
    guru is now called with `-tags "js wasm"` if `syscall/js` is imported in the package


## 18.08.31

* Switch the `ctrl+.`,`ctrl+t` / `cmd+.`,`cmd+t` keybinding to the new &golang.TestCmds{} reducer:

  &golang.TestCmds{
    // additional args to add to the command when running tests and examples
    TestArgs: []string{},

    // additional args to add to the command when running benchmarks
    BenchArgs: []string{"-benchmem"},
  },


## 18.08.29

* implement more aggressive gocode caching.
  behind the scenes, imported/type-checked packages are cached until the respective package is edited.

  * it should now be ok to use `Source: true` option without slowdowns.
  * as a bonus, `go modules` should now have completion with `Source: true`
  * please note that `Source: true` uses a lot more memory (see below for details about cache pruning)
  * if both &golang.Gocode{Source: true} and &golang.GocodeCalltips{Source: true}
    use `Source: true`, they will share the cache (less memory use)

* add new reducer &golang.MarGocodeCtl{}
  this allows manual cache management using the new `margocodectl` command

  * to clear the cache use the command `margocodectl cache-prune`
    run `margocodectl` for info about how to use the command
  * automated cache pruning will be implemented in the future



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



