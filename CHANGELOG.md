## Links:

https://margo.sh/donate - Help support future development of GoSublime.

https://margo.sh/b/hello-margo - A short introduction to margo.

https://margo.sh/b/motd - Get notified when GoSublime has a new release.

---

## Changes

## 18.11.28

This release introduces the HUD and comes with many improvements to snippets and a tweaked version of the Mariana color schemed named Ariana.

- The HUD is an output panel that's automatically populated with info traditionally found in the status bar and various pop-ups/tool-tips.

  Currently, the following info will be displayed there:

  - The `Issues` status, including the error messages for the current line.

  - The `GocodeCalltips` status, including positional highlighting of params and return statement.

  It's bound to the keys `ctrl+.`,`ctrl+0`.

  You can manually bind it to another key. e.g. via `Preferences > Key Bindings`:
  ```json
    {
      "keys": ["ctrl+0"],
      "command": "margo_show_hud",
    },
  ```

- Several new snippets have been added and old ones improved.

  - AppendSnippet: `name = |` suggests:
    - `name = append(name, ...)`
    - `slice = append(slice[:len(slice):len(slice)], ...)`

  - DeferSnippet:
    - `defer func() {}()`
    - `defer f()`

  - ReturnSnippet:
    - `return`

  - MutexSnippet: `mu.|` suggests:
    - `mu.Lock(); defer mu.Unlock(); |`
    - `mu.Lock(); |; mu.Unlock()`

  - PackageNameSnippet:
    - `package main; func main() { | }`

  - DocSnippet: suggest function names, field names, etc. inside the corresponding _documentation_ comment

- Fix the golang.Gocode{} reducer changing View.Pos

- The `(Add)UnimportedPackages` feature no longer adds circular imports

- Show the tasks animation after 500ms instead of 1s to (hopefully) make things feel more responsive

- GOROOT and GOPATH are scanned in parallel at startup

- All packages in GOROOT and GOPATH are available for the unimported packages feature, not just those in GOROOT and the package cache.

- Add better named alias `pkg-list` for `unimported-packages` and show the directory where the package is

- API BREAKAGE:
  Most users should not be affected, but there were some API changes/breakages.

  - removed CursorScope.Any and CursorScope.All

  - make CursorCtx.Pos the int offset instead of token.Pos

  - `CursorNode` and `ParseCursorNode` were removed

  - `CursorCtx.Ctx` is no longer embedded

## 18.11.06

- Fix `UnimportedPackages` support for stdlib packages in go1.10 and earlier versions.

## 18.11.04

- API BREAKAGE: Rename `mg.Reducer.Reducer*` to `mg.Reducer.R*`.

  Most users should be unaffected.
  If you've _called_ any of these methods _directly_,
  you will need to rename the following method calls:

  - ReducerLabel -> RLabel
  - ReducerInit -> RInit
  - ReducerConfig -> RConfig
  - ReducerCond -> RCond
  - ReducerMount -> RMount
  - Reducerduce -> Rduce
  - ReducerUnmount -> RUnmount

- API BREAKAGE: mg.RunCmdData has been un-exported

- The following fields in the `&golang.GoCode{}` and `&golang.GocodeCalltips{}` reducers are now ignored.

  - Source: this is now the default
  - ProposeBuiltins: this is now the default
  - ProposeTests: use `&golang.MarGocodeCtl{}`
  - Autobuild: we now use the source code so there are no plans to implement this
  - UnimportedPackages: this is now the default

  See `&golang.MarGocodeCtl{}` (below).

- Add support for 'unimported' packages.

  - auto-completing `json.` will now try to import `encoding/json`
  - known bugs: when adding the import, the view will scroll
  - known limitation: we don't scan GOPATH and we don't support the vendor directory

  Use `NoUnimportedPackages` (below) to disable this.

- Add support for preloading imported packages when a view is activated.

  - This aims to keep the cache warm to speed up auto-completion.

  Use `NoPreloading` (below) to disable this.

* Add support for adding `unimported` packages to the file.

  - Use `AddUnimportedPackages` (below) to enabled this

* All the above can be configured using the `&golang.MarGocodeCtl{}` reducer

  ```Go
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
  ```

* Add new lang conststants for `mg.JSX`, `mg.TS`, `mg.TSX` and rename `R` to `Rlang`

* Don't treat an empty non-nil slice as matching in `LangIs()` and `ActionIs()`

* Fix an infinite loop when auto-completing inside packages with cyclic dependencies

## 18.10.06

- restore support for running individual test functions by pressing `ctrl+.`,`ctrl+g` or `ctrl+shift + left/right-click` on the function declaration's name

- add support for having multiple builtins with the same name

- API breakage:
  `mg.ExecRunFunc()` was replaced with the pattern `CmdCtx.WithCmd().Run()`
  the former bypasses builtins so running `go install` has no linting support

## 18.09.30

- Improve autocompletion scope detection

  - snippets should now the shown when there is a comment above the package statement
  - completion should no longer be shown when there is no package statement

- misc tweaks to the method snippets
  - for pointer method receivers, only the `*` is selected for the initial method definition
  - when there are syntax errors in the file, methods should no longer be suggested for the invalid type `_`

## 18.09.25

- Switch golang.Gocode and golang.GocodeCalltips to new mode SrcImporterWithFallback by default

  This should improve the experience a lot:

  - in the old `Source: true` mode, CGO packages often failed
  - in the old `Source: false` mode, you had to make sure the package was installed
    and up-to-date
  - in this new mode, we try the more reliable source mode and fallback
    to the binary mode if it fails

  As a result, the `Source: bool` fields are now ignored.
  To restore the old behaviour, use the golang.MarGocodeCtl reducer:

  ```Go
  &golang.MarGocodeCtl{
    ImporterMode: golang.SrcImporterOnly,
    // or
    ImporterMode: golang.BinImporterOnly,
  }
  ```

- replace margocodectl `cache-list-by-key` and `cache-list-by-dur` with `cache-list`
  see `margocodectl cache-list --help`

- Improve FmtCmd's error message

  When goimports fails due to a syntax error, the parse error should now be shown as well
  and not just the meaningless `exit 2` error message

## 18.09.18

- fix a case where margo exits due to IPC shutdown
  _you will need to restart Sublime Text_
- return all possible completions in gocode to allow the editor to do filtering.
  this restores the old behaviour where typing `abc.X` proposes `abc.XYX123`

## 18.09.14

- This release adds a new experimental update notifier.

  MOTD keeps you updated about new versions and important announcements

  It adds a new command `motd.sync` available via the UserCmd palette as `Sync MOTD`

  `Interval` can be set in order to enable automatic update fetching.

  When new updates are found, it displays the message in the status bar
  e.g. `★ margo.sh/cl/18.09.14 ★` a url where you see the upcoming changes before updating

  It sends the following data to the url https://api.margo.sh/motd.json:

  - current editor plugin name e.g. `?client=gosublime`
    this tells us which editor plugin's changelog to check
  - current editor plugin version e.g. `?tag=r18.09.14-1`
    this allows us to determine if there any updates
  - whether or not this is the first request of the day e.g. `?firstHit=1`
    this allows us to get an estimated count of active users without storing
    any personally identifiable data

  No other data is sent. For more info contact privacy at kuroku.io

  To enabled it, add the following reducer:

  ```Go
  &mg.MOTD{
    // Interval, if set, specifies how often to automatically fetch messages from Endpoint
    // Interval: 3600e9, // automatically fetch updates every hour
  },
  ```

  You will need to restart Sublime Text.
  Unless you uncomment/set `Interval`, you will need to manually check for updates
  using the `Sync MOTD` command from the usercmd palette
  `ctrl+.`,`ctrl+c` / `super+.`,`super+c`

- The `GoSublime: Go` syntax was switched to a new syntax based on the Go syntax shipped in Sublime Text

  - if you find any breakages, please file an issue at margo.sh/gs/i
  - if you prefer the colouring of the previous version, you can switch back to the old syntax
    via `Menu > View > Syntax > Open all with current extension as... > GoSublime > GoSublime: Go (Deprecated)`
    please not that this version is buggy and will not receive any fixes

- golang.Gocode, golang.GocodeCalltips:

  - reduce memory use with `Source: true`
  - support syscall/js

- golang.Guru gained support for syscall/js
  guru is now called with `-tags "js wasm"` if `syscall/js` is imported in the package

## 18.08.31

- Switch the `ctrl+.`,`ctrl+t` / `cmd+.`,`cmd+t` keybinding to the new &golang.TestCmds{} reducer:

  ```Go
  &golang.TestCmds{
    // additional args to add to the command when running tests and examples
    TestArgs: []string{},

    // additional args to add to the command when running benchmarks
    BenchArgs: []string{"-benchmem"},
  },
  ```

## 18.08.29

- implement more aggressive gocode caching.
  behind the scenes, imported/type-checked packages are cached until the respective package is edited.

  - it should now be ok to use `Source: true` option without slowdowns.
  - as a bonus, `go modules` should now have completion with `Source: true`
  - please note that `Source: true` uses a lot more memory (see below for details about cache pruning)
  - if both &golang.Gocode{Source: true} and &golang.GocodeCalltips{Source: true}
    use `Source: true`, they will share the cache (less memory use)

- add new reducer `&golang.MarGocodeCtl{}`
  this allows manual cache management using the new `margocodectl` command

  - to clear the cache use the command `margocodectl cache-prune`
    run `margocodectl` for info about how to use the command
  - automated cache pruning will be implemented in the future

## 18.08.22

- merge all shell env vars named `^(MARGO|GO|CGO)\w+` into the GoSublime environment
  this ensures new env vars like `GOPROXY` and `GO111MODULE` work correctly

- try to prevent `GO111MODULE` leaking into the agent build process

- add support for UserCmd prompts

  this enables the creation of UserCmds like the following, without dedicated support from margo:

  ```Go
  mg.UserCmd{
    Title:   "GoRename",
    Name:    "gorename",
    Args:    []string{"-offset={{.View.Filename}}:#{{.View.Pos}}", "-to={{index .Prompts 0}}"},
    Prompts: []string{"New Name"},
  }
  ```

- fix #853 a build failure when using snap packaged go1.10

- fix caching of packages in GOPATH when doing gocode completion
  this _might_ slow completion, but there should no longer be any stale non-GOROOT package completions

- add new `Source` option to use source code for gocode completions
  _this will most likely be very slow_

  ```Go
  &golang.Gocode{ Source: true }
  &golang.GocodeCalltips{ Source: true }
  ```

## 18.08.15

- fix missing `go` command integration by default

- you may need to add the reducer `&golang.GoCmd{}`

- this adds new commands (callable through 9o):

  - `go`: Wrapper around the go command, adding linter support

  - `go.play`: Automatically build and run go commands or run go test for packages
    with support for linting and unsaved files

  - `go.replay`: Wrapper around go.play limited to a single instance
    by default this command is bound to `ctrl+.,ctrl+r` or `cmd+.,cmd+r`

  UserCmds (`ctrl+.,ctrl+c` / `cmd+.,cmd+c`) are also added for `Go Play` and `Go RePlay`
