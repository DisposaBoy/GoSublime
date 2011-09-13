GoSublime
=========

Intro
-----

GoSublime is a plugin for the text editor [SublimeText 2](http://www.sublimetext.com/2).

It provides code completion from [Gocode](https://github.com/nsf/gocode) as well as some context aware snippets to complement the existing SublimeText Go package.

Demo
----

* http://vimeo.com/disposaboy/gosublime-a2-demo

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

You can customize the behaviour of GoSublime by creating a settings file in your `User` package. This can be accessed from within SublimeText by going to the menu `Preferences > Browse Packages...`. Create a file named `GoSublime.sublime-settings`. A sample settings file `GoSublime.sublime-settings.sample` is provided inside the GoSublime package directory.

Note: Filenames are case-sensitive on some platforms(e.g. Linux) so the filename should be exactly `GoSublime.sublime-settings` with capitalization preserved.

e.g. If you want to change the path of gocode, you can edit the `gocode_cmd` setting:

    "gocode_cmd": "/go/bin/gocode"


The available settings are:

* gocode_cmd - the command that shall be called for gocode, this can be the command name e.g. `gocode` or full path to a binary e.g. `/go/bin/gocode` - default `gocode`

* gofmt_cmd - the command that shall be called for gofmt, this can be the command name e.g. `gofmt` or full path to a binary e.g. `/go/bin/gofmt` - default `gofmt`

* run_gofmt_on_save - whether or not to run `gofmt` when the current file is saved - default `false`


Completion Markers
------------------

The following letters are used to prefix entries in the completion list.

* ʂ - Snippets. These snippets are provided by GoSublime and should not be confused with SublimeText's snippets. Furthermore, they are context sensitive, therefore the snippets provided in the global scope will be different to those provided in function scope.

* ν - Variables

* ʈ - User-defined types, structs, maps, slices, etc.

* Ɩ - Constants

* ƒ - Functions

* ρ - Packages


