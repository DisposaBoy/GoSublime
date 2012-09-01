Intro
-----

An experimental remote IDE helper for Go (golang.org) projects.

This project is in the idea/research phase.

It's the Golang part of [GoSublime](https://github.com/DisposaBoy/GoSublime).

If you use [GoSublime](https://github.com/DisposaBoy/GoSublime) then it'll probably just work as I try to keep them both in sync with each other. Otherwise contact me and I may be able to document some things but for now there is intentionally no documentation as things have not yet stablized.

Features (partially/not implemented)
--------

* goto definition of function, variable, etc.
	there are plans to extend this package-wide and possibly system-wide(GOROOT, GOPATH) as well.

* add/remove package imports: the formatting isn't great but it works.

* list all available import paths: simply does a scan of all the directories in GOPATH/GOROOT. it needs to verify that they are actually packages.

* list all packages imported in a file

* return the package name of a file

* godoc support - NOPE

* gofmt support - NOPE

it should be noted that the latter 2 will not be calling `go fmt` nor `go doc` they are intended to simply provide the basic functionality offered by those tools - the aim is to eventually remove all code that's dependant on external commands in GoSublime.

Install
-------

`go get github.com/DisposaBoy/MarGo`