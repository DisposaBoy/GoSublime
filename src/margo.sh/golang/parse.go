package golang

import (
	"go/parser"
	"margo.sh/golang/goutil"
	"margo.sh/mg"
)

const (
	ParseFileMode = goutil.ParseFileMode
)

var (
	NilPkgName   = goutil.NilPkgName
	NilFset      = goutil.NilFset
	NilPkgSrc    = goutil.NilPkgSrc
	NilAstFile   = goutil.NilAstFile
	NilTokenFile = goutil.NilTokenFile
)

type ParsedFile = goutil.ParsedFile

// ParseFile is an alias of goutil.ParseFile
func ParseFile(kvs mg.KVStore, fn string, src []byte) *ParsedFile {
	return goutil.ParseFile(kvs, fn, src)
}

// ParseFileWithMode is an alias of goutil.ParseFileWithMode
func ParseFileWithMode(kvs mg.KVStore, fn string, src []byte, mode parser.Mode) *ParsedFile {
	return goutil.ParseFileWithMode(kvs, fn, src, mode)
}
