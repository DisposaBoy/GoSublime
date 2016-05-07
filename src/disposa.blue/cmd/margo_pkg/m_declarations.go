package margo_pkg

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type Res struct {
	FileDecls []*mDeclarationsDecl `json:"file_decls"`
	PkgDecls  []*mDeclarationsDecl `json:"pkg_decls"`
}

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
	res := &Res{
		FileDecls: []*mDeclarationsDecl{},
		PkgDecls:  []*mDeclarationsDecl{},
	}

	if fset, af, _ := parseAstFile(m.Fn, m.Src, 0); af != nil {
		res.FileDecls = m.collectDecls(fset, af, res.FileDecls)
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
				res.PkgDecls = m.collectDecls(fset, af, res.PkgDecls)
			}
		}
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

func (m *mDeclarations) collectDecls(fset *token.FileSet, af *ast.File, decls []*mDeclarationsDecl) []*mDeclarationsDecl {
	for _, fdecl := range af.Decls {
		if tp := fset.Position(fdecl.Pos()); !tp.IsValid() {
			continue
		}

		switch n := fdecl.(type) {
		case *ast.FuncDecl:
			dd := m.decl(fset, n.Name, "", "func")
			if dd == nil {
				continue
			}

			switch {
			case n.Recv != nil:
				recvFields := n.Recv.List
				if len(recvFields) > 0 {
					typ := recvFields[0].Type
					buf := bytes.NewBufferString("(")
					if printer.Fprint(buf, fset, typ) == nil {
						fmt.Fprintf(buf, ").%s", n.Name.Name)
						dd.Repr = buf.String()
					}
				}
			case dd.Name == "init" && n.Recv == nil:
				dd.Name += " (" + filepath.Base(dd.Fn) + ")"
			}

			decls = append(decls, dd)
		case *ast.GenDecl:
			for _, spec := range n.Specs {
				switch gn := spec.(type) {
				case *ast.TypeSpec:
					if dd := m.decl(fset, gn.Name, "", "type"); dd != nil {
						decls = append(decls, dd)

						switch ts := gn.Type.(type) {
						case *ast.StructType:
							dd.Kind += " struct"
							decls = m.appendFields(decls, fset, ts.Fields, dd)
						case *ast.InterfaceType:
							dd.Kind += " interface"
							decls = m.appendFields(decls, fset, ts.Methods, dd)
						}
					}
				case *ast.ValueSpec:
					for i, v := range gn.Names {
						if vp := fset.Position(v.Pos()); v.Name != "_" && vp.IsValid() {
							switch v.Obj.Kind {
							case ast.Typ, ast.Fun, ast.Con, ast.Var:
								if dd := m.decl(fset, v, "", v.Obj.Kind.String()); dd != nil {
									if v.Obj.Kind == ast.Con && i < len(gn.Values) {
										lit, ok := gn.Values[i].(*ast.BasicLit)
										if ok && lit.Value != "" && len(lit.Value) <= 64 {
											dd.Name += " (" + lit.Value + ")"
										}
									}
									decls = append(decls, dd)
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

func (d *mDeclarations) appendFields(decls []*mDeclarationsDecl, fset *token.FileSet, fields *ast.FieldList, parent *mDeclarationsDecl) []*mDeclarationsDecl {
	// [2:] strip the +/- prefix
	k := parent.Kind[2:]
	if strings.HasPrefix(k, "type ") {
		k = "     " + k[5:]
	}
	for _, f := range fields.List {
		for _, id := range f.Names {
			decls = d.appendDecl(decls, fset, id, parent.Name+": "+id.Name, k)
		}
	}
	return decls
}

func (d *mDeclarations) appendDecl(decls []*mDeclarationsDecl, fset *token.FileSet, id *ast.Ident, name, kind string) []*mDeclarationsDecl {
	if dd := d.decl(fset, id, name, kind); dd != nil {
		return append(decls, dd)
	}
	return decls
}

func (d *mDeclarations) decl(fset *token.FileSet, id *ast.Ident, name, kind string) *mDeclarationsDecl {
	if name == "" {
		name = id.Name
	}

	if name == "_" {
		return nil
	}

	tp := fset.Position(id.Pos())
	if !tp.IsValid() {
		return nil
	}

	return &mDeclarationsDecl{
		Name: name,
		Kind: d.kind(id, kind),
		Fn:   tp.Filename,
		Row:  tp.Line - 1,
		Col:  tp.Column - 1,
	}
}

func (m *mDeclarations) kind(id *ast.Ident, k string) string {
	if id.IsExported() {
		return "+ " + k
	}
	return "- " + k
}
