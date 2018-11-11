# Support

This document aims to clarify what level of support you can expect while using GoSublime.

Use of GoSublime assumes you've read and understood _all_ the points herein.

## Sublime Text

- All versions of Sublime Text **3** should be supported.
- For versions before the official 3.0 release in September 2017, graceful fall-backs are in place.
- Testing is only done for the current non-beta version and only on Linux.

## Experience

- It is assumed that you are experienced with Sublime Text, basic key bindings, its settings system, etc.
- It is assumed that you already have a working Go installation: https://golang.org/doc/install.

## Package Control

- Package Control is not supported

## Go

GoSublime is backed by https://margo.sh/ to which the following points apply:

- Like the official Go [release policy](https://golang.org/doc/devel/release.html#policy), only the current and previous released versions of Go are supported.
- Only the main `gc` tool-chain distributed by https://golang.org/ is supported.
- margo should not require a cgo-enabled Go installation, but non-cgo builds i.e. `CGO_ENABLED=0` are not tested.

## Operating Systems

- Testing is only done on Linux.
- Windows and macOS should work without issue, but no testing is done on them.

## Tools

Please note:

- GoSublime uses its own fork of `gocode` so any installation on your system is ignored.
- By default `fmt` is achieved through direct use of the packages in the stdlib and not the binaries on your system.

## Sponsors & Backers

While we will make an effort to respond to all issues, we have only a limited amount of time and have chosen to give higher priority to our sponsors and backers (including those who donate outside of Open Collective and Patreon).

If an urgent response is required, or an issue has gone without response for more than a few days, our sponsors and backers are welcome to send an email to support@margo.sh.

## Issues with sensitive details

If your issue contains sensitive details or you would otherwise prefer not to post it publicly, you're welcome to send an email to support@margo.sh.
