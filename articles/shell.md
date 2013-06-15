**replace `ctrl` with `super` on OS X**

Shells
======

It's often the case (especially on Linux and OS X), that the environment variables you set in your shell(~/.bashrc) etc. are not visible to  Sublime Text and as a result GoSublime complains about missing GOROOT and/or GOPATH. In order to work around this issue, GoSublime tries to fetch required variables by running your shell. The following points describe how GoSublime decides what your shell is and how to run it.

Note: `${CMD}` or `$CMD` denotes the command that is run through the shell. See the default settings file (`ctrl+dot`,`ctrl+4`) for documentation on the `shell`,`shell_pathsep` and `env` settings

* if the `shell` setting is set, take it as-is
* otherwise, check for the environment variables called `SHELL` or `COMSPEC`. Both variables are checked regardless of platform. They contain the path(preferrably absolute) to your shell.
* If the shell executable path is found, GoSublime takes its basename, so `/bin/bash` becomes `bash` and strips the extension, so `bash.exe` becomes `bash`. For the shells named by `sh`, `bash`, `dash`, `fish`, `zsh` or `rc` the command is `SHELL -l -c ${CMD}`
* If the shell isn't found or is not known then a platform-specific shell is used. On Windows it's `cmd /C ${CMD}` and on Linux and OS X `sh -l -c ${CMD}`
