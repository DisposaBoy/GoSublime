package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"os"
)

type mDeclarations struct {
	Fn     string
	Src    string
	PkgDir string
	Env    map[string]string
}

type mDeclarationsDecl struct {
	Name string `json:"name"`
	Repr string `json:"repr"`
	Kind string `json:"kind"`
	Fn   string `json:"fn"`
	Row  int    `json:"row"`
	Col  int    `json:"col"`
}

func (m *mDeclarations) Call() (interface{}, string) {
	fileDecls := []*mDeclarationsDecl{}
	pkgDecls := []*mDeclarationsDecl{}

	if fset, af, err := parseAstFile(m.Fn, m.Src, 0); err == nil {
		fileDecls = collectDecls(fset, af, fileDecls)
	}

	fset := token.NewFileSet()
	if m.PkgDir != "" {
		var pkgs map[string]*ast.Package

		if fi, err := os.Stat(m.PkgDir); err == nil && fi.IsDir() {
			_, pkgs, _ = parsePkg(fset, m.PkgDir, 0)
		} else {
			_, pkgs, _ = findPkg(fset, m.PkgDir, rootDirs(m.Env), 0)
		}

		for _, pkg := range pkgs {
			for _, af := range pkg.Files {
				pkgDecls = collectDecls(fset, af, pkgDecls)
			}
		}
	}

	res := M{
		"file_decls": fileDecls,
		"pkg_decls":  pkgDecls,
	}

	return res, ""
}

func init() {
	registry.Register("declarations", func(_ *Broker) Caller {
		return &mDeclarations{
			Env: map[string]string{},
		}
	})
}

func collectDecls(fset *token.FileSet, af *ast.File, decls []*mDeclarationsDecl) []*mDeclarationsDecl {
	for _, fdecl := range af.Decls {
		if tp := fset.Position(fdecl.Pos()); tp.IsValid() {
			switch n := fdecl.(type) {
			case *ast.FuncDecl:
				if n.Name.Name != "_" {
					d := &mDeclarationsDecl{
						Name: n.Name.Name,
						Kind: "func",
						Fn:   tp.Filename,
						Row:  tp.Line - 1,
						Col:  tp.Column - 1,
					}

					if n.Recv != nil {
						recvFields := n.Recv.List
						if len(recvFields) > 0 {
							typ := recvFields[0].Type
							buf := bytes.NewBufferString("(")
							if printer.Fprint(buf, fset, typ) == nil {
								fmt.Fprintf(buf, ").%s", n.Name.Name)
								d.Repr = buf.String()
							}
						}
					}

					decls = append(decls, d)
				}
			case *ast.GenDecl:
				for _, spec := range n.Specs {
					switch gn := spec.(type) {
					case *ast.TypeSpec:
						if tp := fset.Position(gn.Pos()); gn.Name.Name != "_" && tp.IsValid() {
							decls = append(decls, &mDeclarationsDecl{
								Name: gn.Name.Name,
								Kind: "type",
								Fn:   tp.Filename,
								Row:  tp.Line - 1,
								Col:  tp.Column - 1,
							})
						}
					case *ast.ValueSpec:
						for _, v := range gn.Names {
							if vp := fset.Position(v.Pos()); v.Name != "_" && vp.IsValid() {
								switch v.Obj.Kind {
								case ast.Typ, ast.Fun, ast.Con, ast.Var:
									decls = append(decls, &mDeclarationsDecl{
										Name: v.Name,
										Kind: v.Obj.Kind.String(),
										Fn:   vp.Filename,
										Row:  vp.Line - 1,
										Col:  vp.Column - 1,
									})
								}
							}
						}
					}
				}
			}
		}
	}
	return decls
}
