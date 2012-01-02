GoSublime
=========

Intro
-----

GoSublime is a Golang plugin collection for the text editor [SublimeText 2](http://www.sublimetext.com/2) providing code completion and other IDE-like features.

Features
--------

* code completion from [Gocode](https://github.com/nsf/gocode)
* context aware snippets via the code-completion popup to complement the existing SublimeText Go package.
* sublime build system(ctrl+b) for gomake, go build, goinstall and [gb](https://github.com/skelterjohn/go-gb) including support for cycling errors with F4/Shift+F4
* gotype support with errors highlighted in the editor view
* gofmt support for quickly reformatting your source to conform with the Go standards
* automatic code completion popup when you press dot(.)

Demo
----

* http://vimeo.com/disposaboy/gosublime-demo2

Installation
------------

1. First (if you haven't done so already) install [Gocode](https://github.com/nsf/gocode).

2. Download GoSublime into your packages directory. You can access your package directory from within SublimeText by going to the menu `Preferences > Browse Packages...`. You can download a zipped archive or tarball on github via the `Downloads` button.
Alternatively (recommended), you can simply clone this repo into your packages directory . (on Linux)

    $ cd ~/.config/sublime-text-2/Packages
    $ git clone git://github.com/DisposaBoy/GoSublime

Usage
-----

Please see USAGE.md for general usage and other tips for effective usage of GoSublime

Settings
--------

You can customize the behaviour of GoSublime by creating a settings file in your `User` package. This can be accessed from within SublimeText by going to the menu `Preferences > Browse Packages...`. Create a file named `GoSublime.sublime-settings` or alternatively copy the default settings file `Packages/GoSublime/GoSublime.sublime-settings` to your `User` package and edit it to your liking.

Note: Filenames are case-sensitive on some platforms(e.g. Linux) so the filename should be exactly `GoSublime.sublime-settings` with capitalization preserved.

Completion Markers
------------------

The following letters are used to prefix entries in the completion list.

* ʂ - Snippets. These snippets are provided by GoSublime and should not be confused with SublimeText's snippets. Furthermore, they are context sensitive, therefore the snippets provided in the global scope will be different to those provided in function scope.

* ν - Variables

* ʈ - User-defined types, structs, maps, slices, etc.

* Ɩ - Constants

* ƒ - Functions

* ρ - Packages


