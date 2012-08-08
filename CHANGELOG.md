## r12.08.08-1
	* fix which could cause MarGo to take a long time to respond (when accidentally parsing binary files)
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
