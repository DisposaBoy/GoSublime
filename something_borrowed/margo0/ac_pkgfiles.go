package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
)

type PkgFilesArgs struct {
	Path string `json:"path"`
}

func init() {
	act(Action{
		Path: "/pkgfiles",
		Doc:  ``,
		Func: func(r Request) (data, error) {
			res := map[string]map[string]string{}
			a := PkgFilesArgs{}
			if err := r.Decode(&a); err != nil {
				return res, err
			}

			srcDir, err := filepath.Abs(a.Path)
			if err != nil {
				return res, err
			}

			fset := token.NewFileSet()
			pkgs, _ := parser.ParseDir(fset, srcDir, isGoFile, parser.PackageClauseOnly)
			if pkgs != nil {
				for pkgName, pkg := range pkgs {
					list := map[string]string{}
					for _, f := range pkg.Files {
						tp := fset.Position(f.Pos())
						if !tp.IsValid() {
							continue
						}

						if _, err := os.Stat(tp.Filename); err != nil {
							continue
						}

						fn, _ := filepath.Rel(srcDir, tp.Filename)
						if fn != "" {
							list[fn] = tp.Filename
						}
					}
					if len(list) > 0 {
						res[pkgName] = list
					}
				}
			}

			return res, nil
		},
	})
}
