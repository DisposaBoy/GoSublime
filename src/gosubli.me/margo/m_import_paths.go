package main

import (
	"errors"
	"go/ast"
	"go/parser"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	registry.Register("import_paths", func(_ *Broker) Caller {
		return &mImportPaths{
			Env: map[string]string{},
		}
	})
}

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

type mImportPathsResponse struct {
	Imports []mImportDecl     `json:"imports"`
	Paths   map[string]string `json:"paths"`
}

func (m *mImportPaths) Call() (interface{}, string) {
	imports, err := m.pkgImports()
	if err != nil {
		imports = []mImportDecl{}
	}
	res := mImportPathsResponse{
		Imports: imports,
		Paths:   m.importPaths(),
	}
	return res, ""
}

func (m *mImportPaths) pkgImports() ([]mImportDecl, error) {
	if m.Fn == "" && m.Src == "" {
		return nil, errors.New("invalid request")
	}
	_, af, err := parseAstFile(m.Fn, m.Src, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	if af == nil || af.Decls == nil {
		return nil, errors.New("invalid fileset")
	}
	var imports []mImportDecl
	for _, decl := range af.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || len(gen.Specs) == 0 {
			continue
		}
		for _, spec := range gen.Specs {
			if imp, ok := spec.(*ast.ImportSpec); ok {
				p := mImportDecl{Path: unquote(imp.Path.Value)}
				if imp.Name != nil {
					p.Name = imp.Name.String()
				}
				imports = append(imports, p)
			}
		}
	}
	return imports, nil
}

func (m *mImportPaths) importPaths() map[string]string {
	// Matching the previous behavior of GoSublime where the
	// environment variables provided by environ and the OS
	// were concatenated.
	roots := uniqueEnv(m.Env)
	w := newImportWalker("")
	for _, root := range roots {
		if m.InstallSuffix == "" {
			w.root = root
		} else {
			w.root = root + "_" + m.InstallSuffix
		}
		filepath.Walk(w.root, w.Walk)
	}
	return w.imports
}

type importWalker struct {
	root string
	// Matching GoSublime's original behavior which returns
	// imports as a map where the keys are the imports and
	// the values are empty string "".
	imports map[string]string
}

func newImportWalker(root string) *importWalker {
	return &importWalker{
		root:    root,
		imports: map[string]string{"unsafe": ""},
	}
}

// Walk appends Go pkg files to importWalker.
func (w *importWalker) Walk(path string, info os.FileInfo, err error) error {
	if err != nil || info.IsDir() {
		return nil
	}
	if !strings.HasSuffix(path, ".a") || strings.HasSuffix(path, "_test.a") {
		return nil
	}
	p, e := filepath.Rel(w.root, path)
	if e != nil {
		return nil
	}
	if strings.HasPrefix(p, ".") || strings.HasPrefix(p, "_") {
		return nil
	}
	p = filepath.Clean(p[:len(p)-2])
	if _, ok := w.imports[p]; !ok {
		// Match GoSublime's original behavior
		// and store the import path in the key
		w.imports[p] = ""
	}
	return nil
}

// uniqueEnv returns a slice of unique Go pkg directories by
// combining the Go paths supplied by env and DefaultEnv.
func uniqueEnv(env map[string]string) []string {
	if env == nil || len(env) == 0 {
		return DefaultEnv.PkgDirs(nil)
	}
	return uniqueSlice(DefaultEnv.PkgDirs(nil), DefaultEnv.PkgDirs(env))
}

// uniqueSlice returns the unique elements of slice s0 and s1.
func uniqueSlice(s0, s1 []string) []string {
	seen := make(map[string]struct{}, len(s0)+len(s1))
	s := make([]string, len(s0)+len(s1))
	n := copy(s, s0)
	copy(s[n:], s1)
	j := 0
	for i := 0; i < len(s); i++ {
		if _, ok := seen[s[i]]; !ok {
			seen[s[i]] = struct{}{}
			s[j] = s[i]
			j++
		}
	}
	return s[:j]
}
