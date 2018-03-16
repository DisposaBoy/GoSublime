package golang

import (
	"disposa.blue/margo/mg"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"io/ioutil"
)

const (
	ParseFileMode = parser.ParseComments | parser.DeclarationErrors | parser.AllErrors
)

var (
	NilFset       = token.NewFileSet()
	NilAstFile, _ = parser.ParseFile(NilFset, "", `package _`, 0)
	NilTokenFile  = NilFset.File(NilAstFile.Pos())
)

type ParsedFile struct {
	Fset      *token.FileSet
	AstFile   *ast.File
	TokenFile *token.File
	Error     error
	ErrorList scanner.ErrorList
}

func ParseFile(sto *mg.Store, fn string, src []byte) *ParsedFile {
	mode := ParseFileMode
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

	type key struct{ hash string }
	k := key{mg.SrcHash(src)}
	if sto != nil {
		if pf, ok := sto.Get(k).(*ParsedFile); ok {
			return pf
		}
	}

	pf := &ParsedFile{Fset: token.NewFileSet()}
	pf.AstFile, pf.Error = parser.ParseFile(pf.Fset, fn, src, mode)
	pf.TokenFile = pf.Fset.File(pf.AstFile.Pos())
	pf.ErrorList, _ = pf.Error.(scanner.ErrorList)
	if pf.AstFile == nil {
		pf.AstFile = NilAstFile
	}
	if pf.TokenFile == nil {
		pf.TokenFile = NilTokenFile
	}

	if sto != nil {
		sto.Put(k, pf)
	}

	return pf
}
