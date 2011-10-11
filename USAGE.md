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


GsLint
------

GsLint is a front-end to `gotype` that highlights errors in the source as you type. Its behaviour can be customized to best suit your style of coding with the following settings.

* gslint_cmd - sets the command to call, it can be the command name e.g. `gotype` or its full path e.g. `/go/bin/gotype`. If you find it distracting or otherwise want to disable it, simply set the value of this setting to an empty string ("")

* gslint_timeout - sets the number of milliseconds to wait after you type a character before calling the command. The default value of 500ms may be too high which causes a noticeable lag from when typing and e.g misspelled(udefined) variables being typed. In that case, set it to something lower like 100 until you feel comfortable with it. Another option is to set it to a value of several seconds, say 3(3000ms) which means it won't activate until you take a rest from typing.

Errors are highlighted as reported by the lint command. In the case of gotype, compile errors reported on line 10 may cause multiple errors on other lines all if which will get highlighted from the point of the error to the end of the line. To see what the error is if it's not immediately clear, move the cursor to the relevant line which will cause the error to be presented in the status bar under the `GsLint: ` marker.


Misc. Helper Commands
---------------------

The following commands can use bound to key bindings to further improve your editing experience.

* gs_commend_forward - this command will activate the ctrl+/ commenting and move the cursor to the next line, allowing you to comment/uncomment multiple lines in sequence without breaking to move the cursor. You can replace the default behaviour by overriding it in your user key bindings(Preferences > Key Bindings - User) with `{ "keys": ["ctrl+/"], "command": "gs_comment_forward" }`

* gs_fmt_save, gs_fmt_prompt_save_as - Due to technical limitations, it's not recommended to run the `gs_fmt` command during the save events(see GsFmt entry above). However, it's possible and may be reasonable to bind to a command that runs `gs_fmt` followed by the `save` command, ensuring any undo's will get saved. For this, two helper commands are provided and can be used to override the default save(`ctrl+s`) and save-as(`ctrl+shift+s`) bindings respectively by adding the following entries to your user key bindings(Preferences > Key Bindings - User): `{ "keys": ["ctrl+s"], "command": "gs_fmt_save" }` and `{ "keys": ["ctrl+shift+s"], "command": "gs_fmt_prompt_save_as" }`.
