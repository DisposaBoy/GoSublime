# asmfmt
Go Assembler Formatter

This will format your assembler code in a similar way that `gofmt` formats your Go code.

Read Introduction: [asmfmt: Go Assembler Formatter](https://blog.klauspost.com/asmfmt-assembler-formatter/)

[![Build Status](https://travis-ci.org/klauspost/asmfmt.svg?branch=master)](https://travis-ci.org/klauspost/asmfmt)
[![Windows Build](https://ci.appveyor.com/api/projects/status/s729ayhkqkjf0ye6/branch/master?svg=true)](https://ci.appveyor.com/project/klauspost/asmfmt/branch/master)
[![GoDoc][1]][2]

[1]: https://godoc.org/github.com/klauspost/asmfmt?status.svg
[2]: https://godoc.org/github.com/klauspost/asmfmt

See [Example 1](https://files.klauspost.com/diff.html), [Example 2](https://files.klauspost.com/diff2.html), [Example 3](https://files.klauspost.com/diff3.html), or compare files in the [testdata folder](https://github.com/klauspost/asmfmt/tree/master/testdata).

Status: STABLE. The format will only change if bugs are found. Please report any feedback in the issue section.

# install

To install the standalone formatter,
`go get -u github.com/klauspost/asmfmt/cmd/asmfmt`

There are also replacements for `gofmt`, `goimports` and `goreturns`, which will process `.s` files alongside your go files when formatting a package.

You can choose which to install:
```
go get -u github.com/klauspost/asmfmt/cmd/gofmt/...
go get -u github.com/klauspost/asmfmt/cmd/goimports/...
go get -u github.com/klauspost/asmfmt/cmd/goreturns/...
```

Note that these require **Go 1.5** due to changes in import paths.

To test if the modified version is used, use `goimports -help`, and the output should look like this:

```
usage: goimports [flags] [path ...]
    [flags]
(this version includes asmfmt)
```

Using `gofmt -w mypackage` will Gofmt your Go files and format all assembler files as well.

# updates

* Aug 8, 2016: Don't indent comments before non-indented instruction.
* Jun 10, 2016: Fixed crash with end-of-line comments that contained an end-of-block `/*` part.
* Apr 14, 2016: Fix end of multiline comments in macro definitions.
* Apr 14, 2016: Updated tools to Go 1.5+
* Dec 21, 2015: Space before semi-colons in macro definitions is now trimmed.
* Dec 21, 2015: Fix line comments in macro definitions (only valid with Go 1.5).
* Dec 17, 2015: Comments are better aligned to the following section.
* Dec 17, 2015: Clean semi-colons in multiple instruction per line.

# emacs

To automatically format assembler, in `.emacs` add:

```
(defun asm-mode-setup ()
  (set (make-local-variable 'gofmt-command) "asmfmt")
  (add-hook 'before-save-hook 'gofmt nil t)
)

(add-hook 'asm-mode-hook 'asm-mode-setup)
```

# usage

`asmfmt [flags] [path ...]`

The flags are similar to `gofmt`, except it will only process `.s` files:
```
	-d
		Do not print reformatted sources to standard output.
		If a file's formatting is different than asmfmt's, print diffs
		to standard output.
	-e
		Print all (including spurious) errors.
	-l
		Do not print reformatted sources to standard output.
		If a file's formatting is different from asmfmt's, print its name
		to standard output.
	-w
		Do not print reformatted sources to standard output.
		If a file's formatting is different from asmfmt's, overwrite it
		with asmfmt's version.
```
You should only run `asmfmt` on files that are assembler files. Assembler files cannot be positively identified, so it will mangle non-assembler files.

# formatting

* Automatic indentation.
* It uses tabs for indentation and blanks for alignment.
* It will remove trailing whitespace.
* It will align the first parameter.
* It will align all comments in a block.
* It will eliminate multiple blank lines.
* Removes `;` at end of line.
* Forced newline before comments, except when preceded by label or another comment.
* Forced newline before labels, except when preceded by comment.
* Labels are on a separate lines, except for comments.
* Retains block breaks (newline between blocks).
* It will convert single line block comments to line comments.
* Line comments have a space after `//`, except if comment starts with `+`.
* There is always a space between parameters.
* Macros in the same file are tracked, and not included in parameter indentation.
* `TEXT`, `DATA` and `GLOBL`, `FUNCDATA`, `PCDATA` and labels are level 0 indentation.
* Aligns `\` in multiline macros.
* Whitespace before separating `;` is removed. Space is inserted after, if followed by another instruction.

