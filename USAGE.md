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
