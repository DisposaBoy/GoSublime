package margo_pkg

import (
	"disposa.blue/margo"
	"disposa.blue/margo/meth/importpaths"
	"go/ast"
	"go/build"
	"go/parser"
	"path/filepath"
)

type mImportPaths struct {
	Fn            string
	Src           string
	Env           map[string]string
	InstallSuffix string
}

type mImportPathsDecl struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (m *mImportPaths) Call() (interface{}, string) {
	imports := []mImportPathsDecl{}
	_, af, err := parseAstFile(m.Fn, m.Src, parser.ImportsOnly)
	if err != nil {
		return M{}, err.Error()
	}

	if m.Fn != "" || m.Src != "" {
		for _, decl := range af.Decls {
			if gdecl, ok := decl.(*ast.GenDecl); ok && len(gdecl.Specs) > 0 {
				for _, spec := range gdecl.Specs {
					if ispec, ok := spec.(*ast.ImportSpec); ok {
						sd := mImportPathsDecl{
							Path: unquote(ispec.Path.Value),
						}
						if ispec.Name != nil {
							sd.Name = ispec.Name.String()
						}
						imports = append(imports, sd)
					}
				}
			}
		}
	}

	bctx := build.Default
	bctx.GOROOT = orString(m.Env["GOROOT"], bctx.GOROOT)
	bctx.GOPATH = orString(m.Env["GOPATH"], bctx.GOPATH)
	bctx.InstallSuffix = m.InstallSuffix
	srcDir, _ := filepath.Split(m.Fn)

	res := M{
		"imports": imports,
		"paths":   margo.Options().ImportPaths(srcDir, &bctx),
	}
	return res, ""
}

func init() {
	margo.Configure(func(o *margo.Opts) {
		if o.ImportPaths == nil {
			o.ImportPaths = importpaths.MakeImportPathsFunc(importpaths.PathFilter)
		}
	})

	registry.Register("import_paths", func(_ *Broker) Caller {
		return &mImportPaths{
			Env: map[string]string{},
		}
	})
}
