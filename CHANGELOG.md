**Donate:**

If you find GoSublime useful and would like to support me and future development of GoSublime,
please donate via one of the available methods on https://github.com/DisposaBoy/GoSublime#donations


**Changes:**

## 18.02.16
	* The new version of margo is close to being ready for real usage.
	  If you'd like to test it out, press `ctrl+.`,`ctrl+x` or `cmd+.`,`cmd+x`
	  to open the extension file and then save it or restart sublime text
	* Highlights:
		* less dependence on Python, so development should be a lot easier going forward
		* it comes with integrated support for GoImports
		* gocode integration now supports more options like autobuild, showing function params and autocompleting packages that have not been imported

## 18.01.17
	* update gocode
	* sync the settings when the active view changes to avoid them going out-of-sync when switching projects
	* add support for exporting env vars into ST.
	  see docs for the setting `export_env_vars` (`ctrl+., ctrl+4`, `super+., super+4` on mac)
	* sync all project settings, not just `env`

## 17.12.17-1
	* fix failure to list some packges in the imports palette
	* update gocode

## 17.12.08-1
	* fix broken commenting when the Go package is disabled

## 17.11.27-1
	* use the old GS syntax definitions instead of the new ones from ST to avoid regressions

## 17.11.25-1
	* use the latest Sublime Text Go syntax
    * convert all our existing syntax definitions to .sublime-synta
    * keep track of the sh.bootstrap output and include it in the Sanity Check

## 17.11.14-1
	* Fix failure to list individual Test|Benchmark|Example functions in the test palette

## 17.11.13-1
	* Change the prefix for identifiers in `*_test.go` files to tilde(~)
	  to prevent cluttering the declartion palette when searching for (un-)exported identifiers

	* Move sh.py/shell bootstrapping into Go and always run it through the shell.

	  This should fix 2 bugs:

	  1. If you had multiple versions of Go installed, one chosen seemingly at random:

	  * Go is installed via your package manager and is setup correctly
	  * You then installed a custom version of Go and set GOROOT (in your shell)
	  * It has no effect even though your custom binary appear first in $PATH

	  This might also fix other cases where the sanity check shows correct settings but compilation fails.

	  2. multi-path GOPATH, PATH, etc. in Fish and similar shells are seemingly ignored

	  In Fish and similar shells where PATH (and other colon separated vars) are lists;
	  when these vars are echoed, they are output as `"dir1" "dir2"` instead of `"dir1":"dir2"`
	  so when GS sees it, it thinks it's 1 non-existend dir `"dir1 dir2"`.

## 17.10.15
	* update gocode
	* fix failure to display `time` (and other packages) in the imports list

## 17.08.23

	* update gocode

## 17.03.05
	* assign default GOPATH as is done in go1.8
	* don't follow symlinks when scanning for package lists

## 17.02.16
	* update gocode

## 16.07.09-1
	* update gocode

## 16.06.02-1
	* update gocode
	* if you're using Go 1.7 beta and experience issues with completion, you're advised to downgrade back to 1.6

## 16.05.07-1
	* Add initial support for MarGo extensions.
	  Press ctrl+.,ctrl+./super+.,super+. and type "Edit MarGo Extension" to open the the extension file.
	  If you your $GOPATH/src contains directories with lots of files or you'd otherwise like
	  to skip when looking up import paths, you can do so by configuring the `ImportPaths` option:

		package gosublime

		import (
			"disposa.blue/margo"
			"disposa.blue/margo/meth/importpaths"
			"path/filepath"
			"strings"
		)

		func init() {
			margo.Configure(func(o *margo.Opts) {
				o.ImportPaths = importpaths.MakeImportPathsFunc(func(path string) bool {
					// note: the default filter includes node_modules

					// use the default filter
					return importpaths.PathFilter(path) &&
						// don't descened into huge node_modules directory
						!strings.Contains(path, filepath.Base("node_modules"))
				})
			})
		}


## 16.05.03-2
	* fallback to the internal MarGo fmt if `fmt_cmd` fails

## 16.05.03-1
	* fix incomplete gocode update

## 16.05.01-1
	* update gocode
	* add struct fields to the declarations palettes

## 16.04.29-1
	* the imports list (ctrl+.,ctrl+p/super+.,super+p) is now sourced from source code packages only
	  and recognises vendored packages

## 16.04.08-2
	* If you use the `fmt_cmd` setting with `goimports` or any other slow command
        you should read and understand the `ipc_timeout` setting documented in `GoSublime.sublime-settings`

## 16.04.08-1
	* added a new SUPPORT.md file calrify what level of support can be expected from use of GoSublime
	* you are advised to reach and understand its contents

## 16.03.22-1
	* add new pseudo env var _dir (`dirname($_fn)`) and do env var substitution on fmt_cmd
	* use `"fmt_cmd": ["goimports", "-srcdir", "$_dir"]` for newer version of goimports

## 16.01.09-1
	* Output GOROOT and GOPATH to the ST console when they change

## 15.12.31-1
	* Update gocode (struct field completion, go15vendor completion, etc.)

## 14.02.25-1
	* added setting `installsuffix`. this should help enabling completion and pkg importing
	  for appengine. set this to `appengine` and add the appengine goroot to your GOPATH e.g.
	  {
	  	 "installsuffix": "appengine",
	     "env": {
	       "GOPATH": "$YOUR_OWN_GOPATH:$PATH_TO_APPENGINE/goroot"
	     }
	  }

	* added setting `ipc_timeout`. if you're experiencing issues with code completion
	  and the error is `Blocking Call(gocode_complete): Timeout`, set this setting to `2`, or `3`, etc..
	  this value is the number of seconds to wait for ipc response from margo before timing out.
	  **note: blocking ipc calls like code completion will freeze the ui if they take too long**

## 13.12.26-1
	* when the key binding `ctrl+dot`,`ctrl+r` is pressed, 9o no longer gains focus

## 13.12.21-2
	* setting `autocomplete_live_hint` was renamed to `calltips` and enabled by default.
		this setting make functions signatures appear in the status bar when you place the
		cursor in a function call
	* the completion_options command was dropped from margo and therefore mg9.completion_options was removed
	* the shell_pathsep setting was removed

## 13.12.21-1
	* make GoSublime's quick panels use a monospace font
	* add a prefix to the declarations panels:
		`+` indicates exported identifiers
		`-` indicates non-exported identifiers
	* in the declarations panels, init functions are now suffixed with ` (filename)`
	* in the declarations panels, const declarations are now suffixed with ` (value)` e.g. `const StatusTeapot (418)`
	* add syntax definitions for the template builtin functions

## 13.12.19-1
	* the OS X key bindings have been removed

	* a copy has been provided below. you may change the "keys" as you wish and place it inside
	    your user key bindings (menu Preferences > Key bindings - User) to restore the functionality

		{
			"keys": ["shift+space"],
			"command": "auto_complete",
			"args": {"disable_auto_insert": true, "api_completions_only": true, "next_completion_if_showing": false},
			"context": [{ "key": "selector", "operator": "equal", "operand": "source.go" }]
		}


## 13.12.17-1
	* give string decoding priority to utf-8 over the system's preferred encoding

## 13.12.15-1
	* remove the ctrl+s, etc. key bindings and fmt the file during the save event.

## 13.12.14-2
	* the autocompletion key bindings on OS X have been changed to shift+space

## 13.12.14-1
	* added new setting `fmt_cmd` to allow replacing margo's fmt with an external gofmt compatible command like like https://github.com/bradfitz/goimports. see the default config for documentation
	* as a last resort, GoSublime will now try to ignore (by replacement) any bytes that cannot be decoded as utf-8 in places that handle strings (like printing to the console)
	* fix the missing `Run {Test,Example,Benchmark}s` entries in the .t palette

## 13.10.05-1
	* sync gocode

## 13.09.07-1
	* remove error syntax highlighting of lone percentage signs in strings

## 13.07.29-1
	* the .p method of finding packages was reverted. as a result `use_named_imports` has no effect

## 13.07.28-1
	* the behaviour of `$GS_GOPATH` has change, please see `Usage & Tips` `ctrl+dot,ctrl+2`
	    section `Per-project  settings & Project-based GOPATH` for details

	* MarGo will now attempt to automatically install packages when you import a package that doesn't exist
	    or when completion fails. see the default settings file, `ctrl+dot,ctrl+4` for more details
	    about the `autoinst` setting

	* a new setting was added to allow using `GS_GOPATH` exclusively. see the default settings file,
	    `ctrl+dot,ctrl+4` for more details on the `use_gs_gopath` setting

	* a new setting to allow importing packages with their package name was added.
	    see the default settings file, `ctrl+dot,ctrl+4` for more details on the `use_named_imports` setting


## 13.07.23-1
	* update gocode

## 13.07.22-1
	* update gocode

## 13.07.17-1
	* the behaviour of 9o output scrolling has changed. instead of attempting to show the end
	    of the output, the start of the output will be shown instead.
	    if you preferred the old behaviour, use the new setting `"9o_show_end": true`

## 13.07.14-1
	* fix comment toggling when the `Go` package is disabled

## 13.07.12-1
	* update gocode

## 13.07.06-2
	* the symbols [ ] ( ) { } , . are now treated as puctuation (they might be syntax highlighted)

## 13.07.06-1
	* the various operator groups, in addition to semi-colons are now treated as `operators` so they should now be syntax highlighted

## 13.07.03-1
	* log MarGo build failure

## 13.07.01-1
	* add user aliases to the 9o completion
	* fix broken arrows keys in 9o completion
	* 9o completion no longer contains the history command prefix (^1 ^2 etc.) (the commands are still shown)

## 13.06.30-4
	* try not to init() GoSublime more than once

## 13.06.30-3
	* the `up` and `down` arrows keys now traverses the 9o history when the cursor is in the prompt

## 13.06.30-2
	* added support for aliases via the setting `9o_aliases`, see the default settings files for documentation

## 13.06.30-1
	This update brings with it a new `GoSublime: Go` syntax definition.
	If you get an error complaining about GoSublime .tmLanguage file,
	you should be able to fix it by closing all `.go` files and restarting Sublime Text.
	If you're using the `GoSublime-next.tmLanguage` please delete the file `Packages/User/GoSublime-next.sublime-settings` (if it exists).
	On update(and restart), all views using syntax files with the base-name `GoSublime.tmLanguage`
	or `GoSublime-next.tmLanguage` will be automatically changed to `GoSublime: Go`.
	Hopefully this change will go smoothly.

	For all other bugs relating to the new syntax definition (e.g. completion stops working)
	please add a comment to https://github.com/DisposaBoy/GoSublime/issues/245

	For all other feature requests or bugs, please open a new issue.

	additionally:

	* there is a new pre-defined variable _nm that is the base name of the current view
	* all pre-defind env vars (_fn, _wd, etc.) are now defined globally and will appear within the
		environment of all 9o command even when run through your shell



## 13.06.29-2
	* show the `go build` output when (re-)installing MarGo
	* show the `go version` output on startup
	* fix the main menu and command palette pointing to the wrong error log file

## 13.06.29-1
	* added 9o `echo` command
	* added two new env vars:
		`$_wd (or $PWD)` contains the 9o working directory
		`$_fn` contains the abs path to the current active view/file (if available)
	* env vars on the 9o command line are expanded before the command is run. see 9o `help`

## 13.06.22-1
	* NOTE: if you have your own GoSublime snippets, the meaning of `global` has changed.
		It will no longer be `true` unless a package is fully declared and the cursor
		is below the line on which the package was declared


## 13.06.16-2
	* added support for automatically setting the `GoSublime: HTML` syntax to certain extensions. See the default settings file (ctrl+dot,ctrl+4) for documentation on the `gohtml_extensions` setting

## 13.06.16-1
	* all undefined 9o commands are now run through your shell. As always, commands can manually be run through the with the `sh` command e.g. `sh echo 123` command

## 13.06.15-1
	* based on the feedback I recieved I integrated with the shell a little...
	* I added support for shells: bash, cygwin/msys/git bash, fish, zsh, rc, etc.
	* see https://github.com/DisposaBoy/GoSublime/blob/master/articles/shell.md for more details

## 13.06.05-1
	* added the shell env var and shell setting to the sanity check output


## 13.06.03-1
	* I added a small article about running [golint](https://github.com/golang/lint) and other user-commands for linting
	  https://github.com/DisposaBoy/GoSublime/blob/master/articles/golint.md

## 13.06.02-1
	* changed GoSublime home dir to Packages/User/GoSublime/[PLATFORM]
	* changed margo exe name to gosublime.margo_[VERSION]_[GO_VERSION].exe
	* sorry for any breakages

## 13.06.01-1
	* fix missing method snippet when using GoSublime-next.tmLanguage

## 13.05.27-3
	* make sure the output panel is always accessed from the main thread

## 13.05.27-2
	* document the `fn_exclude_prefixes` setting

## 13.05.27-1
	* added basic syntax highlighting for go templates (embedded within `{{` and `}}`)
	* inside .go files, `raw` strings(only) now has highlighting for go templates
	      *note* this is only available on the GoSublime-next syntax which will be set to the default
	      for .go files soon, see https://github.com/DisposaBoy/GoSublime/issues/245
	* for html files, the extension .gohtml will yield html highlighting, snippets, etc.
	      as normal .html files do, with addition of go template highighting within `{{` and `}}`
	      see https://github.com/DisposaBoy/GoSublime/issues/252

## 13.05.26-2
	* 9o: `tskill` without args, now opens the `pending tasks` palette

## 13.05.26-1
	* fix `mg -env` in st3 on windows

## 13.05.12-7
	* add basic support for injecting commands into 9o.
		contact me if you intend to make use of this feature

## 13.05.12-6
	* 9o `hist` now honours `9o_instance`

## 13.05.12-5
	* more 9o `cd`, always chdir

## 13.05.12-4
	* fix not being able to properly cd to relative parent directories

## 13.05.12-3
	* add a basic `cd` command to 9o. see 9o `help` for documentation

## 13.05.12-2
	* mg/sh now handless binary output
	* mg/sh now accepts a string `Cmd.Input` that allows passing input the command

## 13.05.12-1
	* improved GoSublime-next syntax highlighting.
	  see https://github.com/DisposaBoy/GoSublime/issues/245

## 13.05.06-4
	* display 9o wd in a simplified manner
	* impl 9o_instance setting: note: this does not work yet

## 13.05.06-3
	* add support for setting the 9o color scheme with `9o_color_scheme`
	* fix completion being shown in gs-next tmlang when the cursor it at the end of the line

## 13.05.06-2
	* disable completion in gs-next: strings, runes, comments

## 13.05.06-1
	* A new syntax definition has been created to fix all the short-comings of the existing
	  syntax highlighting. if you're interested in testing it, please take a look at
	  https://github.com/DisposaBoy/GoSublime/issues/245

## 13.05.04-4
	* add new `9o_instance` and `9o_color_scheme`. note: these are not yet implemented
	  see https://github.com/DisposaBoy/GoSublime/issues/243

## 13.05.04-3
	* add new `lint_enbaled` and `linters` settings. note: the new linter has not yet been implemented
	  see https://github.com/DisposaBoy/GoSublime/issues/220

## 13.05.04-2
	* removed setting: `margo_addr`, if you have a *MarGo* binary in your $PATH, delete it.

## 13.05.04-1
	* don't sort imports when adding/removing

## 13.05.01-2
	* fix mg9 request leak

## 13.05.01-1
	* give PATH preference to Go related bin directories
		this has the side-effect that if you set e.g. GOROOT in your project settings(e.g. GAE),
		then $GOROOT/bin/go should be found first, even if you have a normal Go binary at /usr/bin/gos

## 13.04.27-1
	* fix failure to load GoSublime.tmLanguage in st3

## 13.04.24-1
	* fix gs.which treating directories `$PATH` named `go` as the `go` executable

## 13.04.21-1
	** WARNING **
	**
	** the linter system is being redone
	** this means comp_lint and all lint-related settings will be removed or renamed
	** see https://github.com/DisposaBoy/GoSublime/issues/220
	**

	* only show calltip if the call is on the same line as the cursor:
		this avoids displaying a calltip for fmt.Println() in the following snippet

			fmt.|
			fmt.Println("done")

## 13.04.14-2
	* fix failing to find a calltip for b() in p.a(p.b())

## 13.04.14-1
	* calltips are now implemented in margo

## 13.04.13-1
	* pre-compile margo on update (before restart)
	* detect the go binary's path instead of relying on python
	* try to work-around odd scrolling in 9o

## 13.04.01-1
	* when replaying unsaved views you are now able to navigate to src lines inside 9o

## 13.03.31-2
	* add GOBIIN to PATH
	* set default package snippet to `package main` if the filename is main.go

## 13.03.31-1
	* use relative paths when setting syntax files: should fix any errors about not being able to load e.g. 9o.hidden-tmLanguage

## 13.03.30-3
	* update gocode to https://github.com/nsf/gocode/commit/86e62597306bc1a07d6e64e7d22cd0bb0de78fc3

## 13.03.30-2
	* restore py3k compat: execfile was removed

## 13.03.30-1
	* work-around show_call_tip hang when the file starts with a comment

## 13.03.29-4
	* impl a basic oom killer in MarGo. If MarGo's memory usage reaches 1000m, she? will die
		you can configure this limit in the user settings ctrl+dot,ctrl+5
		e.g. to limit the memory use to 500m use:

			"margo_oom": 500

## 13.03.29-3
	* add support for showing function call tip live in the status bar
		to enable to add the setting:
			"autocomplete_live_hint": true
		to your user settings in ctrl+dot,ctrl+5

		note: the old keybinding ctrl+dot,ctrl+space works as normal

## 13.03.29-2
	* properly detect when the source(about.py) changes
	* notify the user of an update if the on-disk vesion differs from the live version

## 13.03.29-1
	* add bindings for default setting(ctrl+dot,ctrl+4) and user settings(ctrl+dot,ctrl+5)

## 13.03.28-2
	* more python hacks

## 13.03.28-1
	* make the sanity check output more verbose
	* add key bindings for:
		(replace ctrl with super on os x)
		README.md: ctrl+dot,ctrl+1
		USAGE.md: ctrl+dot,ctrl+2
		run sanity check: ctrl+dot,ctrl+3

## r13.03.25-1
	* abort blocking calls(completion, fmt) early if the install stage isn't set to "done"

## r13.03.24-3
	* wait for mg9.install to finish before attempting to send any request to margo.
		fixes a false-positive error about the mg binary being missing before installtion completes

## a13.03.24-2
	* fix call to getcwdu(not in py3k)

## r13.03.24-1
	* communicate a tag/version between gs and mg so the case where they're out-of-sync can be detected


## r13.03.23-2
	* use getcwdu instead of getcwd in hopes of avoiding python issues

## r13.03.23-1
	* foreign platform binaries will no longer be cleaned up
		e.g. where the current platform is linux-x64, a linux-x32 binary (gosublime.margo.r13.03.23-1.linux-x32.exe)
			will not be cleaned up until you load st2 on linux-x32

## r13.03.20-1
	* MarGo EXE name has changed to gosublime.margo.[VERSION].[platform]-[arch].exe
		e.g. gosublime.margo.r13.03.20-1.linux-x64.exe

## r13.03.16-2
	* use the first action on a line if an action is triggered in the wrong place
		e.g. if there is a filename and an error message, clicking on the error message will
		9o will now try find the filename

## r13.03.16-1
	* add imports to the top of the block. this causes them to be a part of the first group of imports
		in cases where imports are group by separating them with a space

## r13.03.03-1
	* reduce false-positives in gs.flag linter
	* fix margo help/flags (remove gocode flags from the output)

## r13.03.02-1
	* cap the number of concurrent requests margo will process

## r13.03.01-1
	* add go/types (like the old gotype) linter
	* disable go/types linter by default:
	    to enabled it, set an empty filter list in your user settings e.g. `"lint_filter": []`

## r13.02.24-1
	* add new setting `lint_filter`. see the default settings for documentation

## r13.02.09-1
	*impl 9o `hist` command

## r13.02.08-2
	* add THANKS.md there are several other donors who weren't added since your donations were
		done anonymously. I'd like to add you as well :) if you want your name added please let me know
		either thank you all!

## r13.02.08-1
	* initial(incomplete) Sublime Text 3 support
	* gsshell ui and gs_shell command removed
	* anything that imported or used gs* commands is probably broken

## r13.02.03-3
	* impl `go share` as 9o command `share`. see ctrl+9 "help" for more details

## r13.02.03-2
	* add new setting `build_command` to allow changing what command is run when you press ctrl+dot,ctrl+b
		see the default settings for documentation (ctrl+dot,ctrl+dot "default settings")

## r13.02.03-1
	* fmt verbs are now highlighted in raw strings
	* fix race between fmt_save and 9o
	* allow action'ing (super/ctrl+g only) seletions in 9o

## r13.01.27-2
	* (by default) only save 9o history if the command was manually executed

## r13.01.27-1
	* correctly handle hist indexing when there's only one command in the history (last command not recalled on ctrl+dot,ctrl+b)

## r13.01.26-1
	* fix broken package snippet (inserting blank package name)

## r13.01.25-2
	* set .go files to use the `GoSublime` syntax definition instead of `Go` as it's more complete
	* hide GsDoc and 9o sytax definitions from the command palette

## r13.01.25-1
	* fix 9o command history indexing (caused wrong command to be expanded for ^1, ^2 etc)

## r13.01.24-2
	* add $HOME/go/bin to $PATH

## r13.01.24-1
	* add $HOME/bin to $PATH

## r13.01.23-1
	* fix broken 9o-related keybindings (ctrl+dot,ctrl+r etc.)

## r13.01.22-1
	* fix missing declarations in unsaved files

## r13.01.21-1
	**majour refactoring - watch out for bugs**

	* fix handling of binary data in the run/replay commands
	* misc tweaks+fixes
	* remove gsdepends
	* remove all rpc calls to margo.py
	* remove margo0

## r13.01.20-1
	**IMPORTANT**
	this update marks the complete transition of all keybindings away from GsShell.
	`ctrl+b` `ctrl+dot`,`ctrl+b` `ctrl+dot`,`ctrl+t` and `ctrl+dot`,`ctrl+r`
	all uses 9o now. for more information about the GsShell replacement 9o please press ctrl+9 and type help

## r13.01.19-2
	**NOTICE**
	The transition to 9o has begun. press ctrl+9 or super+9 and type `help` for more details on 9o.
	9o will evntually completely replace all GoSublime's interaction with the OS' shell.
	This includes GsShell(ctrl+dot,ctrl+b).

	As of this update, `ctrl+dot`,`ctrl+r` and `ctrl+dot`,`ctrl+t` has been remapped

## r13.01.19-1
	* impl 9o command history

## r13.01.17-1
	* add keybindings in 9o for committing autocompletion instead of executing the prompt when auto_complete_commit_on_tab is false

## r13.01.14-1
	* added pledgie badge http://www.pledgie.com/campaigns/19078

## r13.01.12-1

	**WARNING**

		GoSublime will soon switch to 9o `ctrl+9` or `super+9`.
		It will replace GsShell `ctrl+dot`,`ctrl+b` (maybe `ctrl+b`).
		GsShell has reached its EOL and as a result no GsShell specific bugs will be fixed, old or new.
		The code (gsshell.py) will remain for a little while so if you use code that interacts
		with it, now is the time to make let me know so necessary features can implemented in 9o

## r13.01.06-1
	* add two new 9o command `env` and `settings` see 9o `help` for more details
	* 9o now supports a new scheme `gs.packages` e.g. `ctrl+shft`, left-click on gs.packages://GoSublime/9o.md will open the 9o docs

## r13.01.05-2
	* added two task aliases to tskill
		`tskill replay` will kill/cancel the last replay command

		`tskill go` will kill the last go command (go test, etc.). as a consequence,
			the 9o `go` command now acts like the `replay` command in that kills any previous instance

	* added new setting autosave:
		controls whether or not pkg files should be automatically saved when necessary
		(e.g. when running 9o `replay` or `go test` commands)

## r13.01.05-1
	* impl click-tests. i.e `ctrl+shift`,`left-click` on words that start with Test,Benchmark or Example
	will run go corresponding test or bench. `ctrl+shift`,`right-click` will do the same but using only the prefix
	e.g.
		`ctrl+shift`,`left-click` on `BenchmarkNewFunc` will run only `BenchmarkNew`:
			`go test -test.run=none -test.bench="^BenchmarkNew$"`

		`ctrl+shift`,`right-click` on `BenchmarkNewFunc` will run all benchmarks:
			`go test -test.run=none -test.bench="^Benchmark.*"`

## r12.12.29-1
	* impl 9o tskill command. see 9o(ctrl+9) "help" for more info

## r12.12.27-2
	* impl `go test` in 9o run and replay

## r12.12.27-1
	* introducing 9o, the new command-shell. press `ctrl+9` or `super+9` to activate it.
	  WARNING: in the near future 9o will replace GsShell

## r12.12.26-1
	* sync gocode: Windows-specific config_dir/config_file implementation.

## r12.12.13-2
	* add a new setting: `autocomplete_filter_name`
	you may set this to a regexp which will be used to filter entries in the auto-completion list
	e.g. `"autocomplete_filter_name": "^autogenerated_"` will prevent any type or function
	whose name begins with "autogenerated_" from appearing in the auto-completion list

## r12.12.13-1
	* implement `9 replay` command that will `9 play` (build + run) the current package, after killing any previous instances.
	Until it goes live, you can override the existing `ctrl+dot`,`ctrl+r` binding or bind it to something else by adding
	the following key binding to your user key bindings via menu `Preferences > Key Bindings - User`

	{
		"keys": ["ctrl+.", "ctrl+r"],
		"command": "gs_commander_open",
		"args": {"run": ["9", "replay"]},
		"context": [{ "key": "selector", "operator": "equal", "operand": "source.go" }]
	}

## r12.12.2-3
	* setting `margo_cmd` has been removed

## r12.12.2-2
	* setting `gocode_cmd` has been removed

## r12.12.2-1
	* setting `complete_builtins` has been renamed to `autocomplete_builtins`

## r12.11.28-1
	* If you have issues with env vars, particularly on OS X, consider setting the
	`shell` setting. See `Packages/User/GoSublime.sublime-settings` for more details

## r12.11.15-1
	* MarGo (margo0) and gocode are now bundled with GoSublime and should be in active use.
	    Feel free to remove any old source from $GOPATH*/github.com/{nsf/gocode,DisposaBoy/MarGo}
	    if you have no use for them in additiion to their respective binaries

## r12.11.04-1
	* added new setting `complete_builtins`
	  set this to `true` to show builtin type and functions in the completion menu

## r12.11.03-1
	* BREAKING CHANGES ARE COMING: in the next GoSublime update support for windows-style
	    environment variables will be removed.
	    If you have environment variables that are not expanded before GS sees them and they are
	    of the form `%HOME%`, `%GOPATH%` etc. they will no longer be expanded.
	    You should transition to *nix-style env vars.

	    i.e `%GOPATH%` etc. should be changed to `$GOPATH`. `$$` can be used to escape to escape`$` characters

## r12.09.22-1
	* the experimental gsshell replacement codename shelly is no more.
	    it has been replaced with gscommander which operates from within the output panel
	    a separate instance per directory

	    to activate it press `ctrl+#` or `super+#`
	    proper documentation will appear as things progress but for now it works as follows:
	        paths are highlighted(usually bold), pressing `ctrl+shift+[left click]` will open it.
	        if the path is a file it will open in ST2 otherwise if it's a url it will be opened
	        in your web browser

	        typing `#` followed by a command and pressing `enter` will run that command

	        auto-completion and implementation of common commands such as `cd` and `go play` will follow soon


## r12.09.16-1
	* add typename-aware method definition snippets for types declared in the current file

## r12.09.08-2
	* add new setting `comp_lint_commands` that allows you specify what commands comp-lint should run
	    e.g to run `go vet` followed by `go install`, add the following to your user settings.
	    by default the only command run for comp-lint is `go install`

		"comp_lint_enabled": true, // enable comp-lint
		"comp_lint_commands": [
			{"cmd": ["go", "install"]}, // first run go install
			{"cmd": ["go", "vet"]}      // followed by go vet,
		],
		"on_save": [
			{"cmd": "gs_comp_lint"} // setup comp-lint to run after you save a file
		],

	    see `Package/GoSublime/GoSublime.sublime-settings` for details

## r12.09.08-1
	* add support snippets (an alternative to Sublime Text's Native snippets)
	    see `Package/GoSublime/GoSublime.sublime-settings` for details

## r12.08.26-1
	* make gs_browse_files (`ctrl+dot`,`ctrl+m`) act more like a file browser.
	    it now lists all files in the current directory tree excluding known binary files:
	        (.exe, .a, files without extension, etc.) and an entry to go to the parent directory

## r12.08.23-1
	* add experimental support for post-save commands
	    a new entry `on_save` is supported in `GoSublime.sublime-settings`, it takes a list of commands
	    in the form of an object {"cmd": "...", "args": {...}} where cmd can be any TextCommand
	* add experimental support for `go install` on save in the form of another linter.
	to activate it add the following to your `GoSublime.sublime-settings`

	    "comp_lint_enabled": true,
	    "on_save": [
	        {"cmd": "gs_comp_lint"}
	    ]

	note: enabling this will override(disable) the regular GsLint

## r12.08.10-3
	* `ctrl+dot`,`ctrl+a` is now accessible globally

## r12.08.10-2
	* `ctrl+dot`,`ctrl+o` now presents a file list instead of opening a file

## r12.08.10-1
	* `ctrl+dot`,`ctrl+m` now list all relevant files (.go, .c, etc.)
	    as well all files in the directory tree recursively (sub-packages)
	    it also now works globally

## r12.08.08-1
	* fix a bug which could cause MarGo to take a long time to respond (when accidentally parsing binary files)
	update MarGo

## r12.07.31-1
	* add platform info e.g (linux, amd64) to pkg declarations list (`ctrl+dot`,`ctrl+l`)

## r12.07.28-2
	* add command palette entry to show the build output
	    press `ctrl+dot`,`ctrl+dot` and start typing `build output`

## r12.07.28-1
	* update gocode: nsf fixed a bug that could cause gocode to hang on invalid input

## r12.07.21-2
	* fix: handle filename for browse files correctly
	    update MarGo

## r12.07.21-1
	* add support for browsing/listing the files in a the current package
	    press `ctrl+dot`,`ctrl+m`
	    update MarGo

## r12.07.15-2
	* add basic call-tip? support
	* press `ctrl+dot`,`ctrl+space` inside a function parameter list to show its declaration

## r12.07.15-1
	* update gocode: nsf recently added improved support for variables declared in the head of `if` and `for` statements

## r12.07.12-1
	* fix: imports not sorted on fmt/save
	* fix: GsDoc doesn't work correctly in unsaved files
	* various presentation tweaks
	* documentation comments are now displayed for types
	* package documentation is now displayed
	* goto definition of packages is now enabled
	* various keybindings now available in non .go files
	    `ctrl+dot`,`ctrl+dot` - open the command palette with only GoSublime entries
	    `ctrl+dot`,`ctrl+n` - create a new .go file
	    `ctrl+dot`,`ctrl+o` - browse packages
	* update MarGo

## r12.07.08-2
	* new quick panel for go test
	    allows easily running `Test.*`, `Example.*`, `Benchmark.*` or individual tests, examples and benchmarks
	    press `ctrl+dot`,`ctrl+t` to access the quick panel

## r12.07.08-1
	* you can now browse packages
	    press `ctrl+dot`,`ctrl+o` to open the first file found in the select pkg dir
	* new key binding added `ctrl+dot`,`ctrl+l` to list the declarations in the current pkg in a single step
	    it does the same thing as `ctrl+dot`,`ctrl+a` and then selecting 'Current Package'

## r12.07.07-2
	* you can now browse declarations in the current package(beyond file-scope)
	      as well as all other packages
	      press `ctrl+dot`,`ctrl+a` to browser packages via a quick panel
	      listing the declarations in the current is still `ctrl+dot+`,`ctrl+d`
	* update MarGo

## r12.07.07-1
	* improve GsLint detection of un-called flag.Parse()
	* listing declarations now works in unsaved files
	* please update MarGo

## r12.06.29-2
	* GsDoc documentation now shows example functions and blocks are now collapsed
	* update MarGo

## r12.06.29-1
	* fix: threading that caused gslint to crash
	*
	* added initial support for per-project settings
	*     a settings object named `GoSublime` in your project settings will override values
	*     specified in the `Gosublime.sublime-settings` file
	*
	* added new dynamic pseudo-environment variable `GS_GOPATH` will contain an auto-detected GOPATH
	*     e.g. if you file name is `/tmp/go/src/hello/main.go` it will contain the value `/tmp/go`
	*     it can safely added to your regular `GOPATH` `env` setting e.g.
	*     `"env": { "GOPATH": "$HOME/go:$GS_GOPATH" }`
	*     this allows for seemless use of project-based GOPATHs without explicit configuration
	*
	* added ctrl+click binding for GsDoc
	*     `ctrl+shift+left-click` acts as alias for `ctrl+dot,ctrl+g` a.k.a goto definition
	*     `ctrl+shift+right-click` acts as alias for `ctrl+dot,ctrl+h` a.k.a show documentation hint
	*     as always, `super` replace `ctrl` on OS X

## r12.06.26-2
	* GsDoc now supports local, package-global and imported package variables and functions
		(MarGo/doc is still incomplete, however: types(structs, etc.) are not resolved yet)
		I've changed the way GsDoc works. Both mode are unified, ctrl+dot,ctrl+g will take you to
		the definition but the hint( ctrl+dot,ctrl+h ) now displays the src along with any comments
		attached to it (this is usually pure documentation)
	* MarGo needs updating

## r12.06.26-1
	* fix: file saving in gsshell
	* fix: duplicating comment that follows imports when imports are modified
	* fix: adding duplicate entries to the package list due to filename case-insensitivity
	* the new_go_file command now automatically fills out the package declaration
	* add binding to create a new go file ( ctrl+dot,ctrl+n )

## r12.06.17-1
	* add support for running(play) the current file without saving it (`ctrl+dot`, `ctrl+r`)
	* add support for sharing the contents of the current on play.golang.org
	press `ctrl+dot`, `ctrl+dot` for a list of all commands and their key bindings as well sharing functionality

## r12.06.09-2
	* MarGo now supports warning about calling flag.String() etc and forgetting to call flag.Parse() afterwards

## r12.06.09-1
	* removed ctrl+shift+g keybinding, please use `ctrl+dot`,`ctrl+dot` to show the list of available commands and their kebindings
	* complete implementation of imports:
	      use `ctrl+dot`,`ctrl+p` to add or remove packages
	      use `ctrl+dot`,`ctrl+i` to quickly jump to the last imported package where you can assign an alias, etc.
	      use `ctrl+dot`,`ctrl+[` to go back to where you were before
	* MarGo needs updating and a restart of ST2 is recommended

## r12.06.05-1
	* add support for configuring the fmt tab settings - see GoSublime.sublime-settings (fmt_tab_width and fmt_tab_indent)

## r12.06.02-1
	* Add initial stub implementation of goto-definition and show-documentation
	*     this requires the latest version of MarGo
	* new key bindings and commands: press `ctrl+.`, `ctrl+.`
	*     (control or super on OS X, followed by .(dot) twice)
	*     or open the command palette(`ctrl+shift+p`) and type `GoSublime:`
	*     to show a list of available commands and their respective key bindings
	* note: currently only the pkgname.Function is supported, so types, methods or constants, etc.

## r12.05.30-1
	* fix completion only offering the 'import snippet' if there aren't any imports in the file

## r12.05.29-1
	* update MarGo

## r12.05.26-2
	* re-enable linting

## r12.05.26-1
	* start using margo.fmt, no more dependecy on `gofmt` and `diff`

## r12.05.05-1
	* add support for installing/updating Gocode and MarGo
