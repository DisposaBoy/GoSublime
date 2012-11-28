Usage
=====

Note
----

* Unless otherwise mentioned, `super` replaces `ctrl` in keybindings on OS X.
* A mention of a (GO)PATH variable uses a colon(`:`) as the sepator.
This is the PATH separator on Linux and OS X, Windows uses a semi-colon(`;`)

Settings
--------

You may customize GoSublimes behaviour by (creating and) customizing the file file `Packages/User/GoSublime.sublime-settings`. Default settings are documented in `Packages/GoSublime/GoSublime.sublime-settings`. **WARNING** Do not edit any package file outside of `Packages/User/`, including files inside `Packages/GoSublime/` unless you have a reason to. These files are subject to being overritten on update of the respective package and/or Sublime Text itself. You may also inadvertently prevent the respective package from being able to update via git etc.

Quirks
------

In some system environment variables are not passed around as expected.
The result of which is that some commands e.g `go build` doesn't work
as the command cannot be found or `GOPATH` is not set. To get around this
the simplest thing to do is to set these variables in the settings file.
See the documention for the `env` and/or `shell` setting, both documented in the default
settings file `Packages/User/GoSublime.sublime-settings`

Code Completion
---------------

Completion can be accessed by typing the (default) key combination `CTRL+[SPACE]` inside a Golang file.

Key Bindings
------------

By default, a number of key bindings are provided. They can be viewed in by opening the command palette
and typing `GoSublime:` or via the key binding `ctrl+dot,ctrl+dot` (or super+dot,super+dot on OS X).
Wherever I refer to a key binding with `ctrl+` it is by default defined as `super+` on OS X unless stated otherwise.

Useful key bindings
-------------------

Often when commenting out a line the immediate action following this is to move the cursor to the next line either to continue working or comment out the following line.

With this following key binding, you can have the line commented out and the cursor automatically moved to the next line.

{ "keys": ["ctrl+/"], "command": "gs_comment_forward", "context": [{ "key": "selector", "operator": "equal", "operand": "source.go" }] },

Package Imports
---------------

pressing `ctrl+dot,ctrl+p` will open the package list from which you can quickly import or delete a package import.
The usage is the same for both operations. If the package is already imported then it will appear near the top
and be marked as a *delete* operation, so it effect is a toggle. If you want to edit the alias of a package e.g
a database driver: first import the package as normal and then press `ctrl+dot,ctrl+i` to quicky jump
the last imported package. Once edited you can return to where you were by pressing `ctrl+dot,ctrl+[`

Build System
------------

A Go build system is provided under the menu `Tools > Build System > GsShell`. If you're using this build system, when you press `ctrl+b` you will get a prompt for the command you want to run. The command can be any valid command e.g `go build && pkill MarGo; ./MarGo` or `go run *.go`. The prompt is automatically filled with `go ` and pressing tab will try to complete some of the common `go` subcommands. So typing `go b` then pressing tab results in `go build`. If the prompt is empty or only contains `go` (ignoring whitespace) then when you press tab, it will instead be replaced with the last command you ran.

Build/Test/Go Command
---------------------

A mini-shell-alike prompt is provided in the form of GsShell. It allows you to type any command your normal SHELL(bash, etc.) will accept.
Therefore all the normal go tools are available. Some go commands are *enhanced* to provide a more streamlined environment. A command `go play` is added as a direct alias of `go run`. It will run the current file. `*.go` is treated as special and the .go files in the current
directly that are not _test.go files are passed to the corresponding command. The last command ran is remembered so you don't
have to type it out again next time. When the promp appears, you may press `tab` to fill out the prompt with the last command.
Additionally some command alias are available via tabbing e.g typing `go t[tab]` will expand to `go test`, `go b[tab]` expands to `go build` and so on.

To access GsShell you can press `ctrl+dot,ctrl+b` or alternatetively access it by setting you Go build system to GsShell at which point it will become available via `ctrl+b` as well. Pressing `ctrl+r` will invoke the `go play`(go run) command via GsShell.

Per-project  settings & Project-based GOPATH
------------------------------

If you have a settings object called `GoSublime` in your project settings its values will override those
inside the `GoSublime.sublime-settings` file. As a side-effect you may set a specific GOPATH for a simple
project.

If the only setting you use this functionality to change is the GOPATH, then you may be able to find
success by adding the string `$GS_GOPATH` to your global `GOPATH` setting e.g.
`"env": { "GOPATH": "$HOME/go:$GS_GOPATH" }`
This is a pseudo-environment-variable.
It's changed to match all a possible GOPATH based on the current file name. e.g. if your file name is
`/tmp/go/src/hello/main.go` then it will contain the value `/tmp/go`

Lint/Syntax Check
-----------------

The source is continuously scanned for syntax errors. It will try to catch some common errors, like
forgetting to call flag.Parse (if this causes false positives, please file a bug report).

Apart from the hilighting in the view using a dot icon in the gutter and usually underlining the
first character of an error region. You are given an entry in the status bar in the form: `GsLint (N)`
where `N` is the number of errors found in that file. You can show the list of errors and navigate to
then by pressing `ctrl+dot,ctrl+e`. Errors for the current line are shown in the status bar.

Fmt
---

by default `ctrl+s` and `ctrl+shift+s` are overridden to fmt the the file before saving it. You may also
fmt a file without saving it by pressing `ctrl+dot,ctrl+f`

Godoc/Goto Definition
---------------------

To show the source and associated comments(documentation) of a variable press `ctrl+dot,ctrl+h` or
using the mouse `ctrl+shift,right-click`. This will show an output panel that presents the full
definition of the variable or function under the (first) cursor along with its comments.
To goto the definition instead, press `ctrl+dot,ctrl+g` or alternatively using the mouse `ctrl+shift,left-click`.

Declarations/Code Outline?
--------------------------

A very minimal form of code outline is provided. You can press `ctrl+dot,ctrl+d` to list all the declartions
in the current file.

New File
--------

Pressing `ctrl+dot,ctrl+n` will create a new Go file with the package declaration filled out.
It will try to be intelligent about it, so if the current directory contains package `mypkg` it will use that as the package name.

Misc. Helper Commands
---------------------

The following commands can use bound to key bindings to further improve your editing experience.

* gs_fmt - this command runs `gofmt` on the current buffer and also available via the key bindings `ctrl+dot,ctrl+f`

* gs_fmt_save, gs_fmt_prompt_save_as - these commands will run the `go_fmt` followed `save` or `prompt_save_as` - these are bound to `ctrl+s` and `ctrl+shift+s` respectively by default

* gs_comment_forward - this command will activate the ctrl+/ commenting and move the cursor to the next line, allowing you to comment/uncomment multiple lines in sequence without breaking to move the cursor. You can replace the default behaviour by overriding it in your user key bindings(Preferences > Key Bindings - User) with `{ "keys": ["ctrl+/"], "command": "gs_comment_forward", "context": [{ "key": "selector", "operator": "equal", "operand": "source.go" }] }`
