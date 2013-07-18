Internally in MarGo is a goroutine to handle automatic installs by reading `AutoInstOptions` from a channel.
It'll need to have some throttling because we don't want to end up executing `go install` a lot.
We also don't want to be installing the same pkg over and over so pkg existense is checked before running `go install`

Iff `go install` succeeds(results in a `.a` file) then a message is sent back to the client.
In this case GS can just display a message in the status bar or something.


	type AutoInstOptions struct {
		// if ImportPaths is empty, Src is parsed in order to populate it
		ImportPaths []string
		Src         string

		// the environment variables as passed by the client - they should not be merged with os.Environ(...)
		// GOPATH is be valid
		Env map[string]string
	}

* MarGo/m_gocode:
	if completion fails to find anything, send `Src` and `Env`, then return just like it does now

* MarGo/m_imports:
	when a pkg is added send `ImportPaths` and `Env`

* on-save:
	this one is probably best served by the linter or an `on_save` command, that way you get feedback abut compile errors etc.

There is a potential issue with concurrent installs. These can come about because MG is running an auto-install.
Meanwhile, the client just triggered a save event.
There's no way to determine when the user is calling `go install` because it can come via `go test -i`, some shell command, etc.
Currently, GS tries to work around this by assuming(hoping) that install is always run through `9o`.
If this is the case and the user hasn't changed the `go` command or run it through their shell,
then every second call to the `go` command kills the previous which is often enough.

Possible side-effects of concurrent install:

* a second compile instance fails to open or rename the `.a` file because it's already open
* high memory and CPU use due to having multiple compiles going at the same time
