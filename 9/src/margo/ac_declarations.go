package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"os"
)

type DeclarationsArgs struct {
	Fn     string            `json:"filename"`
	Src    string            `json:"src"`
	PkgDir string            `json:"pkg_dir"`
	Env    map[string]string `json:"env"`
}

type DeclarationsRes struct {
	FileDecls []*Decl `json:"file_decls"`
	PkgDecls  []*Decl `json:"pkg_decls"`
}

type Decl struct {
	Name string `json:"name"`
	Repr string `json:"repr"`
	Kind string `json:"kind"`
	Fn   string `json:"fn"`
	Row  int    `json:"row"`
	Col  int    `json:"col"`
}

func init() {
	act(Action{
		Path: "/declarations",
		Doc:  "",
		Func: func(r Request) (data, error) {
			a := DeclarationsArgs{}
			res := DeclarationsRes{
				FileDecls: []*Decl{},
				PkgDecls:  []*Decl{},
			}

			if err := r.Decode(&a); err != nil {
				return res, err
			}

			if fset, af, err := parseAstFile(a.Fn, a.Src, 0); err == nil {
				res.FileDecls = collectDecls(fset, af, res.FileDecls)
			}

			fset := token.NewFileSet()
			if a.PkgDir != "" {
				var pkgs map[string]*ast.Package

				if fi, err := os.Stat(a.PkgDir); err == nil && fi.IsDir() {
					_, pkgs, _ = parsePkg(fset, a.PkgDir, 0)
				} else {
					_, pkgs, _ = findPkg(fset, a.PkgDir, rootDirs(a.Env), 0)
				}

				for _, pkg := range pkgs {
					for _, af := range pkg.Files {
						res.PkgDecls = collectDecls(fset, af, res.PkgDecls)
					}
				}
			}

			return res, nil
		},
	})
}

func collectDecls(fset *token.FileSet, af *ast.File, decls []*Decl) []*Decl {
	exists := map[string]bool{}
	for _, fdecl := range af.Decls {
		if tp := fset.Position(fdecl.Pos()); tp.IsValid() {
			x, ok := exists[tp.Filename]
			if !ok {
				_, err := os.Stat(tp.Filename)
				x = err == nil
				exists[tp.Filename] = x
			}
			if !x {
				continue
			}

			switch n := fdecl.(type) {
			case *ast.FuncDecl:
				if n.Name.Name != "_" {
					d := &Decl{
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
							decls = append(decls, &Decl{
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
									decls = append(decls, &Decl{
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
