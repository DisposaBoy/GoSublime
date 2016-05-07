This document aims clarify what level of support you can expect when using GoSublime.
Use of GoSublime assumes you've read and understood *all* the points herein.

Discussion of support and this file in particular are tracked here: https://github.com/DisposaBoy/GoSublime/issues/689



# Sublime Text

* **Sublime Text 2 is *not* supported.**
* It's several years old at this point and Sublime HQ does not respond to my support requests.
* Furthermore, they've changed the default download link to Sublime Text 3 implying they do not support it either.

If you have a *good* reason to not upgrade to Sublime Text 3,
discuss it here https://github.com/DisposaBoy/GoSublime/issues/689



# Experience

* It is assumed that you are experienced with Sublime Text, basic key bindings, its settings system, etc.
* It is assumed that you already have a working Go installation: https://golang.org/doc/install
* You are expect to have read and understand the contents of the files: GoSublime.sublime-settings, USAGE.md and 9o.md

# Sublime Text's Go package

* I disable the built-in Go package so I do not test for compatibility or conflicts with GoSublime.

# Package Control

* I do not use Package Control and therefore not able to offer support for any issue related to it.
* As a user, *you* are expected take care when updating GoSublime.
* You are advised *not* to utomatically update GoSublime.

# Go

Please not that GoSublime is backed by a Go program named MarGo to which the following points apply.

* The minimum supported version of Go is go1.6.
* Older versions of Go might be able to compile MarGo without issue, but I will not test these older versions.
* I also do not test the gccgo, llvm, etc. tool-chains. Only the main `gc` tool-chain is supported.
* MarGo should not require a cgo-enabled Go installation, but I do not test installations with it disabled.

# Operating Systems

* I only test Linux.
* Windows and OS X should work without issue, but I do *not* test anything on them.

# Tools

Please note:

* GoSublime uses its own fork of `gocode` so any installation on your system is ignored.
* By default `fmt` is achieved through direct use of the packages in the stdlib and not the binaries on your system.

I do not use the following tools and do *not* test for compatibility with them:

* GVM or any other Go version manager
* GB or any other other alternative to the `go` tool
* `goimports`, the `gofmt`/`go fmt` *binary* or any other `gofmt` alternative
* If you use the `fmt_cmd` setting with `goimports` or any other slow command
   you should read and understand the `ipc_timeout` setting documented in `GoSublime.sublime-settings`
