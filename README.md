

[![Backers on Open Collective](https://opencollective.com/gosublime/backers/badge.svg)](#backers) [![Sponsors on Open Collective](https://opencollective.com/gosublime/sponsors/badge.svg)](#sponsors) [![Build Status](https://travis-ci.org/DisposaBoy/GoSublime.svg?branch=master)](https://travis-ci.org/DisposaBoy/GoSublime)
<hr>

GoSublime
=========

Intro
-----

GoSublime is a Golang plugin collection for the text editor [Sublime Text](http://www.sublimetext.com/) providing code completion and other IDE-like features. Only Sublime Text **3** is supported.

Before using GoSublime you should read and understand [SUPPORT.md](https://github.com/DisposaBoy/GoSublime/blob/master/SUPPORT.md)

Features
--------

* code completion from [Gocode](https://github.com/nsf/gocode)
* context aware snippets via the code-completion popup to complement the existing SublimeText Go package.
* sublime build system(ctrl+b) integrating with GoSublime 9o command prompt
* lint/syntax check as you type
* quickly jump to any syntax error reported (and jump back to where you were before (across files))
* quickly fmt your source or automatically on save to conform with the Go standards
* easily create a new go file and run it without needing to save it first (9o `replay`)
* share your snippets (anything in the loaded file) on play.golang.org
* list declarations in the current file
* automatically add/remove package imports
* quickly jump your import section(automatically goes to the last import) where you can easily edit the pkg alias and return to where you were before
* go to definition of a package function or constant, etc.
* show the source(and thus documentation) of a variable without needing to change views

Demo
----

* Old demo http://vimeo.com/disposaboy/gosublime-demo2

![](https://github.com/DisposaBoy/GoSublime/raw/master/ss/2.png)
![](https://github.com/DisposaBoy/GoSublime/raw/master/ss/1.png)

Installation
------------

It is assumed that you have a working installation of [Git](https://git-scm.com/) and know how to use it to clone and update repositories.

Run the command `git clone https://github.com/DisposaBoy/GoSublime` from within the Sublime Text `Packages` directory.
The location of your Sublime Text Packages directory can be found by clicking the menu: `Preferences` > `Browse Packages...`.

Usage
-----

Please see [USAGE.md](USAGE.md) and [9o.md](9o.md) for general usage and other tips for effective usage of GoSublime

**NOTE** GoCode is entirely integrated into GoSublime/MarGo. If you see any bugs related to completion,
assume they are GoSublime's bugs and I will forward bug reports as necessary.

Settings
--------

You can customize the behaviour of GoSublime by creating a settings file in your `User` package. This can be accessed from within SublimeText by going to the menu `Preferences > Browse Packages...`. Create a file named `GoSublime.sublime-settings` or alternatively copy the default settings file `Packages/GoSublime/GoSublime.sublime-settings` to your `User` package and edit it to your liking.

Note: File names are case-sensitive on some platforms (e.g. Linux) so the file name should be exactly `GoSublime.sublime-settings` with capitalization preserved.


Copyright, License & Contributors
=================================

GoSublime and MarGo are released under the MIT license. See [LICENSE.md](LICENSE.md)

GoSublime is the copyrighted work of *The GoSublime Authors* i.e me ([https://github.com/DisposaBoy/GoSublime](DisposaBoy)) and *all* contributors. If you submit a change, be it documentation or code, so long as it's committed to GoSublime's history I consider you a contributor. See [AUTHORS.md](AUTHORS.md) for a list of all the GoSublime authors/contributors.

Thanks to all the people who contribute. [[Contribute](CONTRIBUTING.md)].
<a href="graphs/contributors"><img src="https://opencollective.com/gosublime/contributors.svg?width=890" /></a>

Supporters
==========

GoSublime has received support from many kind individuals and as a thank you I've added most to [THANKS.md](THANKS.md) file as a way of saying *Thank You*. Some donors donated anonymously and so are not listed, however. If you have donated and would like to add an entry to this file, feel free to open a pull request.

Donations
=========

Supporting me means I can spend more time working on GoSublime and other Open Source projects, hopefully leading to more consistent and regular development.

Donate using Liberapay

<a href="https://liberapay.com/DisposaBoy/donate"><img alt="Donate using Liberapay" src="https://liberapay.com/assets/widgets/donate.svg"></a>



Donate using PayPal

<a href="https://www.paypal.com/cgi-bin/webscr?cmd=_s-xclick&hosted_button_id=4RFMYNTYTUQJU"><img alt="Donate using PayPal" src="https://www.paypalobjects.com/en_GB/i/btn/btn_donate_LG.gif"/></a>





Become a backer or a sponsor on OpenCollective

### Backers

Thank you to all our backers! 🙏 [[Become a backer](https://opencollective.com/gosublime#backer)]

<a href="https://opencollective.com/gosublime#backers" target="_blank"><img src="https://opencollective.com/gosublime/backers.svg?width=890"></a>


### Sponsors

Support this project by becoming a sponsor. Your logo will show up here with a link to your website. [[Become a sponsor](https://opencollective.com/gosublime#sponsor)]

<a href="https://opencollective.com/gosublime/sponsor/0/website" target="_blank"><img src="https://opencollective.com/gosublime/sponsor/0/avatar.svg"></a>
<a href="https://opencollective.com/gosublime/sponsor/1/website" target="_blank"><img src="https://opencollective.com/gosublime/sponsor/1/avatar.svg"></a>
<a href="https://opencollective.com/gosublime/sponsor/2/website" target="_blank"><img src="https://opencollective.com/gosublime/sponsor/2/avatar.svg"></a>
<a href="https://opencollective.com/gosublime/sponsor/3/website" target="_blank"><img src="https://opencollective.com/gosublime/sponsor/3/avatar.svg"></a>
<a href="https://opencollective.com/gosublime/sponsor/4/website" target="_blank"><img src="https://opencollective.com/gosublime/sponsor/4/avatar.svg"></a>
<a href="https://opencollective.com/gosublime/sponsor/5/website" target="_blank"><img src="https://opencollective.com/gosublime/sponsor/5/avatar.svg"></a>
<a href="https://opencollective.com/gosublime/sponsor/6/website" target="_blank"><img src="https://opencollective.com/gosublime/sponsor/6/avatar.svg"></a>
<a href="https://opencollective.com/gosublime/sponsor/7/website" target="_blank"><img src="https://opencollective.com/gosublime/sponsor/7/avatar.svg"></a>
<a href="https://opencollective.com/gosublime/sponsor/8/website" target="_blank"><img src="https://opencollective.com/gosublime/sponsor/8/avatar.svg"></a>
<a href="https://opencollective.com/gosublime/sponsor/9/website" target="_blank"><img src="https://opencollective.com/gosublime/sponsor/9/avatar.svg"></a>


