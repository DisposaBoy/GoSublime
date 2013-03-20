GoSublime Changes
-----------------

Note: you may need to restart Sublime Text after GoSublime updates

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
