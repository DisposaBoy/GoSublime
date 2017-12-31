package margo_pkg

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type Doc struct {
	Src  string `json:"src"`
	Pkg  string `json:"pkg"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	Fn   string `json:"fn"`
	Row  int    `json:"row"`
	Col  int    `json:"col"`
}

type mDoc struct {
	Fn        string
	Src       string
	Env       map[string]string
	Offset    int
	TabIndent bool
	TabWidth  int
}

func (m *mDoc) Call() (interface{}, string) {
	res := []*Doc{}

	fset, af, err := parseAstFile(m.Fn, m.Src, parser.ParseComments)
	if err != nil {
		return res, err.Error()
	}

	sel, id := identAt(fset, af, m.Offset)
	if id == nil {
		return res, ""
	}

	pkgs, _ := parser.ParseDir(fset, filepath.Dir(m.Fn), fiHasGoExt, parser.ParseComments)
	if pkgs == nil {
		pkgs = map[string]*ast.Package{}
	}

	pkgName := af.Name.Name
	files := map[string]*ast.File{}
	pkg, _ := pkgs[pkgName]
	if pkg != nil {
		files = pkg.Files
	}
	files[m.Fn] = af
	pkg, _ = ast.NewPackage(fset, files, nil, nil)
	if pkg == nil {
		return res, ""
	}
	pkgs[pkg.Name] = pkg

	obj, pkg, objPkgs := findUnderlyingObj(fset, af, pkg, pkgs, rootDirs(m.Env), sel, id)
	if obj != nil {
		res = append(res, objDoc(fset, pkg, m.TabIndent, m.TabWidth, obj))
		if objPkgs != nil {
			xName := "Example" + obj.Name
			xPrefix := xName + "_"
			for _, objPkg := range objPkgs {
				xPkg, _ := ast.NewPackage(fset, objPkg.Files, nil, nil)
				if xPkg == nil || xPkg.Scope == nil {
					continue
				}

				for _, xObj := range xPkg.Scope.Objects {
					if xObj.Name == xName || strings.HasPrefix(xObj.Name, xPrefix) {
						res = append(res, objDoc(fset, xPkg, m.TabIndent, m.TabWidth, xObj))
					}
				}
			}
		}
	}
	return res, ""
}

func init() {
	registry.Register("doc", func(_ *Broker) Caller {
		return &mDoc{
			Env: map[string]string{},
		}
	})
}

func objDoc(fset *token.FileSet, pkg *ast.Package, tabIndent bool, tabWidth int, obj *ast.Object) *Doc {
	decl := obj.Decl
	kind := obj.Kind.String()
	tp := fset.Position(obj.Pos())
	objSrc := ""
	pkgName := ""
	if pkg != nil && pkg.Name != "builtin" {
		pkgName = pkg.Name
	}

	if obj.Kind == ast.Pkg {
		pkgName = ""
		doc := ""
		// special-case `package name` is generated as a TypeSpec
		if v, ok := obj.Decl.(*ast.TypeSpec); ok && v.Doc != nil {
			doc = "/*\n" + v.Doc.Text() + "\n*/\n"
		}
		objSrc = doc + "package " + obj.Name
	} else if af, ok := pkg.Files[tp.Filename]; ok {
		switch decl.(type) {
		case *ast.TypeSpec, *ast.ValueSpec, *ast.Field:
			line := tp.Line - 1
			for _, cg := range af.Comments {
				cgp := fset.Position(cg.End())
				if cgp.Filename == tp.Filename && cgp.Line == line {
					switch v := decl.(type) {
					case *ast.TypeSpec:
						v.Doc = cg
					case *ast.ValueSpec:
						v.Doc = cg
					case *ast.Field:
						pkgName = ""
						kind = "field"
					}
					break
				}
			}
		}
	}

	if objSrc == "" {
		objSrc, _ = printSrc(fset, decl, tabIndent, tabWidth)
	}

	return &Doc{
		Src:  objSrc,
		Pkg:  pkgName,
		Name: obj.Name,
		Kind: kind,
		Fn:   tp.Filename,
		Row:  tp.Line - 1,
		Col:  tp.Column - 1,
	}
}

func isBetween(n, start, end int) bool {
	return (n >= start && n <= end)
}

func identAt(fset *token.FileSet, af *ast.File, offset int) (sel *ast.SelectorExpr, id *ast.Ident) {
	ast.Inspect(af, func(n ast.Node) bool {
		if n != nil {
			start := fset.Position(n.Pos())
			end := fset.Position(n.End())
			if isBetween(offset, start.Offset, end.Offset) {
				switch v := n.(type) {
				case *ast.SelectorExpr:
					sel = v
				case *ast.Ident:
					id = v
				}
			}
		}
		return true
	})
	return
}

func findUnderlyingObj(fset *token.FileSet, af *ast.File, pkg *ast.Package, pkgs map[string]*ast.Package, srcRootDirs []string, sel *ast.SelectorExpr, id *ast.Ident) (*ast.Object, *ast.Package, map[string]*ast.Package) {
	if id != nil && id.Obj != nil {
		return id.Obj, pkg, pkgs
	}

	if id == nil {
		// can this ever happen?
		return nil, pkg, pkgs
	}

	if sel == nil {
		if obj := pkg.Scope.Lookup(id.Name); obj != nil {
			return obj, pkg, pkgs
		}
		fn := filepath.Join(runtime.GOROOT(), SrcPkg, "builtin")
		if pkgBuiltin, _, err := parsePkg(fset, fn, parser.ParseComments); err == nil {
			if obj := pkgBuiltin.Scope.Lookup(id.Name); obj != nil {
				return obj, pkgBuiltin, pkgs
			}
		}
	}

	if sel == nil {
		return nil, pkg, pkgs
	}

	switch x := sel.X.(type) {
	case *ast.Ident:
		if x.Obj != nil {
			// todo: resolve type
		} else {
			if v := pkg.Scope.Lookup(id.Name); v != nil {
				// todo: found a type?
			} else {
				// it's most likely a package
				// todo: handle .dot imports
				for _, ispec := range af.Imports {
					importPath := unquote(ispec.Path.Value)
					pkgAlias := ""
					if ispec.Name == nil {
						_, pkgAlias = path.Split(importPath)
					} else {
						pkgAlias = ispec.Name.Name
					}
					if pkgAlias == x.Name {
						if id == x {
							// where do we go as the first place of a package?
							pkg, pkgs, _ = findPkg(fset, importPath, srcRootDirs, parser.ParseComments|parser.PackageClauseOnly)
							if pkg != nil {
								// we'll just match the behaviour of package browsing
								// we will visit some file within the package
								// but which file, or where is undefined
								var f *ast.File
								ok := false
								if len(pkg.Files) > 0 {
									basedir := ""
									for fn, _ := range pkg.Files {
										basedir = filepath.Dir(fn)
										break
									}
									baseFn := func(fn string) string {
										return filepath.Join(basedir, fn)
									}
									if f, ok = pkg.Files[baseFn("doc.go")]; !ok {
										if f, ok = pkg.Files[baseFn("main.go")]; !ok {
											if f, ok = pkg.Files[baseFn(pkgAlias+".go")]; !ok {
												// try to keep things consistent
												filenames := sort.StringSlice{}
												for filename, _ := range pkg.Files {
													filenames = append(filenames, filename)
												}
												sort.Sort(filenames)
												f = pkg.Files[filenames[0]]
											}
										}
									}
								}

								if f != nil && f.Name != nil {
									doc := f.Doc
									if len(f.Comments) > 0 && f.Doc == nil {
										doc = f.Comments[len(f.Comments)-1]
									}
									o := &ast.Object{
										Kind: ast.Pkg,
										Name: f.Name.Name,
										Decl: &ast.TypeSpec{
											Name: f.Name,
											Doc:  doc,
											Type: f.Name,
										},
									}
									return o, pkg, pkgs
								}
							}
							// in-case we don't find a pkg decl
							return nil, pkg, pkgs
						}

						if pkg, pkgs, _ = findPkg(fset, importPath, srcRootDirs, parser.ParseComments); pkg != nil {
							obj := pkg.Scope.Lookup(id.Name)
							return obj, pkg, pkgs
						}
					}
				}
			}
		}
	}
	return nil, pkg, pkgs
}
