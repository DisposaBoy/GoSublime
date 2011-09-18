Usage
=====

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
            "file": "Packages/GoSublime/macros/Dot.sublime-macro",
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
    }

A sample file is provided in `Packages/GoSublime/examples/Default.sublime-keymap.example`, you can simply copy or symlink it to your `Packages/User` directory.

Build System
------------

The gomake build system enables Sublime Text 2 to recognize the 5g/6g/8g output so you can jump to compile errors by clicking on the output or cycle through them by using F4/Shift+F4.

If you want to use the gomake build system you will have to copy the file `Packages/GoSublime/examples/Gomake.sublime-build.example` to `Packages/GoSublime/Gomake.sublime-build`.

If gomake is not in your system path you will have to add the following key/value pair to `Packages/GoSublime/Gomake.sublime-build`:

"path": "/path/to/go/bin:$PATH",

Gofmt
-----

GoSublime provides a text command `gs_fmt` which formats the current file using gofmt.

You can utilize this command by adding a key binding, e.g. by clicking the menu`Preferences > Key Bindings - User` in Sublime Text 2 and adding the following entry:

    { "keys": ["ctrl+shift+e"], "command": "gs_fmt" }

which will call the gs_fmt command whenever you press `Ctrl+Shift+E`. You can set the key binding to anything you prefer, however, it's not recommended to bind it to `Ctrl+S` which also saves the file because the buffer must be edited and in the unlikely event that the patch fails, the changes are undone leaving the file on-disk with the erroneous changes.
