package goutil

import (
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"io/ioutil"
	"margo.sh/mg"
)

const (
	ParseFileMode = parser.ParseComments | parser.DeclarationErrors | parser.AllErrors
)

var (
	NilPkgName    = "_"
	NilFset       = token.NewFileSet()
	NilPkgSrc     = "\n\npackage " + NilPkgName + "\n"
	NilAstFile, _ = parser.ParseFile(NilFset, "", NilPkgSrc, 0)
	NilTokenFile  = NilFset.File(NilAstFile.Pos())
)

type ParsedFile struct {
	Fset      *token.FileSet
	AstFile   *ast.File
	TokenFile *token.File
	Error     error
	ErrorList scanner.ErrorList
}

func ParseFile(kvs mg.KVStore, fn string, src []byte) *ParsedFile {
	return ParseFileWithMode(kvs, fn, src, ParseFileMode)
}

func ParseFileWithMode(kvs mg.KVStore, fn string, src []byte, mode parser.Mode) *ParsedFile {
	if len(src) == 0 {
		var err error
		if fn != "" {
			src, err = ioutil.ReadFile(fn)
		}
		if len(src) == 0 {
			return &ParsedFile{
				Fset:      NilFset,
				AstFile:   NilAstFile,
				TokenFile: NilTokenFile,
				Error:     err,
			}
		}
	}

	type key struct {
		hash string
		mode parser.Mode
	}
	k := key{hash: mg.SrcHash(src), mode: mode}
	if kvs != nil {
		if pf, ok := kvs.Get(k).(*ParsedFile); ok {
			return pf
		}
	}

	pf := &ParsedFile{Fset: token.NewFileSet()}
	pf.AstFile, pf.Error = parser.ParseFile(pf.Fset, fn, src, mode)
	if pf.AstFile == nil {
		pf.AstFile = NilAstFile
	}
	pf.TokenFile = pf.Fset.File(pf.AstFile.Pos())
	if pf.TokenFile == nil {
		pf.TokenFile = NilTokenFile
	}
	pf.ErrorList, _ = pf.Error.(scanner.ErrorList)

	if kvs != nil {
		kvs.Put(k, pf)
	}

	return pf
}
