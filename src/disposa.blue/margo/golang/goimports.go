package golang

import (
	"golang.org/x/tools/imports"
)

var (
	GoImports = &fmter{fmt: func(fn string, src []byte) ([]byte, error) {
		return imports.Process(fn, src, nil)
	}}
)
