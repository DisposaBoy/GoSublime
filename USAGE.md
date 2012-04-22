Usage
=====

Settings
--------

You may customize GoSublimes behaviour by modiying its default settings. Default settings are documented in `Packages/GoSublime/GoSublime.sublime-settings`

Quirks
------

In some system environment variables are not passed around as expected.
The result of which is that some command e.g `go build` doesn't work
as the command cannot be found or `GOPATH` is not set. To get around this
the simplest thing to do is to set these variables in the settings file.
See the documention for the `env` setting in the `GoSublime.sublime-settings`

Code Completion
---------------

Completion can be accessed by typing the (default) key combination `CTRL+[SPACE]` inside a Golang file.

Key Bindings
------------

By default, a number of key bindings are provided. They can be viewed in `Packages/GoSublime/Default.sublime-keymap`

Useful key bindings
-------------------

It's useful to have the autocomplete popup up as soon as you hit the dot key.

You may achieve this by adding this to your `Packages/User/Default.sublime-keymap` file:

    {
        "keys": ["."],
        "command": "run_macro_file",
        "args":
        {
            "file": "Packages/GoSublime/macros/DotCompletion.sublime-macro"
        },
        "context":
        [
            {
                "key": "auto_complete_visible",
                "operator": "equal",
                "operand": true
            },
            {
                "key": "selector",
                "operator": "equal",
                "operand": "source.go"
            }
        ]
    },
    {
        "keys": ["."],
        "command": "run_macro_file",
        "args":
        {
            "file": "Packages/GoSublime/macros/Dot.sublime-macro"
        },
        "context":
        [
            {
                "key": "auto_complete_visible",
                "operator": "equal",
                "operand": false
            },
            {
                "key": "selector",
                "operator": "equal",
                "operand": "source.go"
            }
        ]
    }


Often when commenting out a line the immediate action following this is to move the cursor to the next line either to continue working or comment out the following line.

With this following key binding, you can have the line commented out and the cursor automatically moved to the next line.

{ "keys": ["ctrl+/"], "command": "gs_comment_forward", "context": [{ "key": "selector", "operator": "equal", "operand": "source.go" }] },

Build System
------------

A Go build system is provided under the menu `Tools > Build System > GsShell`. If you're using this build system, when you press `ctrl+b` you will get a prompt for the command you want to run. The command can be any valid command e.g `go build && pkill MarGo; ./MarGo` or `go run *.go`. The prompt is automatically filled with `go ` and pressing tab will try to complete some of the common `go` subcommands. So typing `go b` then pressing tab results in `go build`. If the prompt is empty or only contains `go` (ignoring whitespace) then when you press tab, it will instead be replaced with the last command you ran.

In addition to GsShell, a number of example build system configs are provided which covers gomake, go build, goinstall and [gb](https://github.com/skelterjohn/go-gb).

The gomake build system enables Sublime Text 2 to recognize the 5g/6g/8g output so you can jump to compile errors by clicking on the output or cycle through them by using F4/Shift+F4.

If you want to use the gomake build system you will have to copy the file `Packages/GoSublime/examples/Gomake.sublime-build.example` to `Packages/GoSublime/Gomake.sublime-build`.

If gomake is not in your system path you will have to add the following key/value pair to `Packages/GoSublime/Gomake.sublime-build`:

"path": "/path/to/go/bin:$PATH",

The instructions above apply to all example build system configs in `Packages/GoSublime/examples/[BUILD SYSTEM].sublime-build.example`.

GsLint
------

GsLint is a front-end to `gotype` and similar commands. It highlights errors in the source as you type. Errors are highlighted as reported by the lint command. In the case of gotype, compile errors reported on line 10 may cause multiple errors. Each line with an error will be marked by a bookmark icon(arrow-like) in the gutter and the first character of the invalid code will be underlined. e.g. an undefined variable `name` when passed to an existing function `println` will result in the letter `n` being underlined. To see what the error is, if it's not immediately clear, move the cursor to the relevant line which will cause the error to be displayed in the status bar under the `GsLint: ` marker.

GsPalette
---------

The GsPalette is a quick panel allowing you to quickly jump to errors identified by GsLint and back to the previous position of cursor. The default key binding is `ctrl+shift+g`.

Misc. Helper Commands
---------------------

The following commands can use bound to key bindings to further improve your editing experience.

* gs_fmt - this command runs `gofmt` on the current buffer.

* gs_fmt_save, gs_fmt_prompt_save_as - these commands will run the `go_fmt` followed `save` or `prompt_save_as` - these are bound to `ctrl+s` and `ctrl+shift+s` respectively by default

* gs_comment_forward - this command will activate the ctrl+/ commenting and move the cursor to the next line, allowing you to comment/uncomment multiple lines in sequence without breaking to move the cursor. You can replace the default behaviour by overriding it in your user key bindings(Preferences > Key Bindings - User) with `{ "keys": ["ctrl+/"], "command": "gs_comment_forward", "context": [{ "key": "selector", "operator": "equal", "operand": "source.go" }] }`
