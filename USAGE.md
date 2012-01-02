Usage
=====

Settings
--------

You may customize GoSublimes behaviour by modiying its default settings. Default settings are documented in `Packages/GoSublime/GoSublime.sublime-settings`


Code Completion
---------------

Completion can be accessed by typing the (default) key combination `CTRL+[SPACE]` inside a Golang file.

Automatic/Dot Completion
------------------------

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

A sample file is provided in `Packages/GoSublime/examples/Default.sublime-keymap.example`, you can simply copy or symlink it to your `Packages/User` directory.

Build System
------------

A number of build system configs are provided which covers gomake, go build, goinstall and [gb](https://github.com/skelterjohn/go-gb).

The gomake build system enables Sublime Text 2 to recognize the 5g/6g/8g output so you can jump to compile errors by clicking on the output or cycle through them by using F4/Shift+F4.

If you want to use the gomake build system you will have to copy the file `Packages/GoSublime/examples/Gomake.sublime-build.example` to `Packages/GoSublime/Gomake.sublime-build`.

If gomake is not in your system path you will have to add the following key/value pair to `Packages/GoSublime/Gomake.sublime-build`:

"path": "/path/to/go/bin:$PATH",

The instructions above apply to all example build system configs in `Packages/GoSublime/examples/[BUILD SYSTEM].sublime-build.example`.

Gofmt
-----

GoSublime provides a text command `gs_fmt` which formats the current file using gofmt.

You can utilize this command by adding a key binding, e.g. by clicking the menu`Preferences > Key Bindings - User` in Sublime Text 2 and adding the following entry:

    { "keys": ["ctrl+shift+e"], "command": "gs_fmt", "context": [{ "key": "selector", "operator": "equal", "operand": "source.go" }] }

which will call the gs_fmt command whenever you press `Ctrl+Shift+E`. You can set the key binding to anything you prefer, however, it's not recommended to bind it to `Ctrl+S` which also saves the file because the buffer must be edited and in the unlikely event that the patch fails, the changes are undone leaving the file on-disk with the erroneous changes.


GsLint
------

GsLint is a front-end to `gotype` and similar commands. It highlights errors in the source as you type. Errors are highlighted as reported by the lint command. In the case of gotype, compile errors reported on line 10 may cause multiple errors. Each line with an error will be marked by a `x` in the gutter and the first character of the invalid code will be underlined. e.g. an undefined variable `name` when passed to an existing function `println` will result in the letter `n` being underlined. To see what the error is, if it's not immediately clear, move the cursor to the relevant line which will cause the error to be displayed in the status bar under the `GsLint: ` marker.


Misc. Helper Commands
---------------------

The following commands can use bound to key bindings to further improve your editing experience.

* gs_commend_forward - this command will activate the ctrl+/ commenting and move the cursor to the next line, allowing you to comment/uncomment multiple lines in sequence without breaking to move the cursor. You can replace the default behaviour by overriding it in your user key bindings(Preferences > Key Bindings - User) with `{ "keys": ["ctrl+/"], "command": "gs_comment_forward", "context": [{ "key": "selector", "operator": "equal", "operand": "source.go" }] }`

* gs_fmt_save, gs_fmt_prompt_save_as - Due to technical limitations, it's not recommended to run the `gs_fmt` command during the save events(see GsFmt entry above). However, it's possible and may be reasonable to bind to a command that runs `gs_fmt` followed by the `save` command, ensuring any undo's will get saved. For this, two helper commands are provided and can be used to override the default save(`ctrl+s`) and save-as(`ctrl+shift+s`) bindings respectively by adding the following entries to your user key bindings(Preferences > Key Bindings - User): `{ "keys": ["ctrl+s"], "command": "gs_fmt_save", "context": [{ "key": "selector", "operator": "equal", "operand": "source.go" }] }` and `{ "keys": ["ctrl+shift+s"], "command": "gs_fmt_prompt_save_as", "context": [{ "key": "selector", "operator": "equal", "operand": "source.go" }] }`.
