package main

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
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
	paths := importsPaths(srcDir, &bctx)

	res := M{
		"imports": imports,
		"paths":   paths,
	}
	return res, ""
}

func init() {
	registry.Register("import_paths", func(_ *Broker) Caller {
		return &mImportPaths{
			Env: map[string]string{},
		}
	})
}

func importPaths(environ map[string]string, installSuffix string) ([]string, error) {
	imports := []string{
		"unsafe",
	}
	paths := map[string]bool{}

	env := []string{
		environ["GOPATH"],
		environ["GOROOT"],
		os.Getenv("GOPATH"),
		os.Getenv("GOROOT"),
		runtime.GOROOT(),
	}
	for _, ent := range env {
		for _, path := range filepath.SplitList(ent) {
			if path != "" {
				paths[path] = true
			}
		}
	}

	seen := map[string]bool{}
	pfx := strings.HasPrefix
	sfx := strings.HasSuffix
	osArchSfx := osArch
	if installSuffix != "" {
		osArchSfx += "_" + installSuffix
	}
	for root, _ := range paths {
		root = filepath.Join(root, "pkg", osArchSfx)
		walkF := func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				p, e := filepath.Rel(root, p)
				if e == nil && sfx(p, ".a") {
					p := p[:len(p)-2]
					if !pfx(p, ".") && !pfx(p, "_") && !sfx(p, "_test") {
						p = path.Clean(filepath.ToSlash(p))
						if !seen[p] {
							seen[p] = true
							imports = append(imports, p)
						}
					}
				}
			}
			return nil
		}
		filepath.Walk(root, walkF)
	}
	return imports, nil
}

func importsPaths(srcDir string, bctx *build.Context) map[string]string {
	rootDirs := bctx.SrcDirs()

	importDir := func(dir string) *build.Package {
		p := quickImportDir(bctx, rootDirs, dir)
		if p != nil && p.Name != "" && p.ImportPath != "" {
			return p
		}
		return nil
	}

	srcImportPath := quickImportPath(rootDirs, srcDir)

	var pkgs []*build.Package
	for _, dir := range rootDirs {
		pkgs = append(pkgs, ImportablePackages(dir, importDir)...)
	}

	res := make(map[string]string, len(pkgs))
	res["unsafe"] = "" // this package doesn't exist on-disk

	const vdir = "/vendor/"
	var vendored []*build.Package
	for _, p := range pkgs {
		switch {
		case p.Name == "main":
		// it's rarely useful to import `package main`
		case p.ImportPath == "builtin":
		// this package exists for documentation only
		case strings.HasPrefix(p.ImportPath, vdir[1:]) || strings.Contains(p.ImportPath, vdir):
			// fill these in after everything else so we can tag them
			vendored = append(vendored, p)
		default:
			res[p.ImportPath] = importsName(p)
		}
	}
	if srcImportPath != "" {
		sfx := srcImportPath + "/"
		for _, p := range vendored {
			name := importsName(p) + " [vendored]"
			ipath := p.ImportPath
			vpos := strings.LastIndex(ipath, vdir)
			switch {
			case vpos > 0:
				pfx := ipath[:vpos+1]
				if strings.HasPrefix(sfx, pfx) {
					ipath := ipath[vpos+len(vdir):]
					res[ipath] = name
				}
			case strings.HasPrefix(ipath, vdir[1:]):
				ipath := ipath[len(vdir)-1:]
				res[ipath] = name
			}
		}
	}
	return res
}

func quickImportPath(rootDirs []string, srcDir string) string {
	for _, rootDir := range rootDirs {
		dir := strings.TrimPrefix(srcDir, rootDir)
		if dir != srcDir && strings.HasPrefix(dir, string(filepath.Separator)) {
			return filepath.ToSlash(dir[1:])
		}
	}
	return ""
}

func quickImportDir(bctx *build.Context, rootDirs []string, srcDir string) *build.Package {
	ipath := quickImportPath(rootDirs, srcDir)
	if ipath == "" {
		return nil
	}

	f, err := os.Open(srcDir)
	if err != nil {
		return nil
	}
	defer f.Close()

	pkg := &build.Package{
		Dir:        srcDir,
		ImportPath: ipath,
	}

	fset := token.NewFileSet()
	for {
		names, _ := f.Readdirnames(100)
		if len(names) == 0 {
			break
		}
		for _, nm := range names {
			ignore := !strings.HasSuffix(nm, ".go") ||
				strings.HasSuffix(nm, "_test.go") ||
				strings.HasPrefix(nm, ".") ||
				strings.HasPrefix(nm, "_")

			if ignore {
				continue
			}

			path := filepath.Join(srcDir, nm)
			mode := parser.PackageClauseOnly | parser.ParseComments
			af, _ := parser.ParseFile(fset, path, nil, mode)
			pkg.Name = astFileName(af)
			if pkg.Name != "" {
				return pkg
			}
		}
	}
	return nil
}

func astFileName(af *ast.File) string {
	if af == nil || af.Name == nil {
		return ""
	}
	name := af.Name.String()
	if name == "" || name == "documentation" {
		return ""
	}
	for _, g := range af.Comments {
		for _, c := range g.List {
			for _, ln := range strings.Split(c.Text, "\n") {
				i := strings.Index(ln, "+build:")
				if i >= 0 && strings.Index(" "+ln[i:]+" ", " ignore ") > 0 {
					return ""
				}
			}
		}
	}
	return name
}

func importsName(p *build.Package) string {
	return p.Name
}

func ImportablePackages(root string, importDir func(path string) *build.Package) []*build.Package {
	dirs := allDirNames(root, func(nm string) bool { return !ignoreNames(nm) })
	var ents []*build.Package
	for _, dir := range dirs {
		if p := importDir(dir); p != nil {
			ents = append(ents, p)
		}
	}
	return ents
}

func ignoreNames(nm string) bool {
	return strings.HasPrefix(nm, ".") ||
		strings.HasPrefix(nm, "_") ||
		nm == "testdata" ||
		knownFileExts[filepath.Ext(nm)]
}

func allDirNames(dir string, match func(basename string) bool) []string {
	dirs := dirNames(dir, match)
	for _, dir := range dirs {
		dirs = append(dirs, allDirNames(dir, match)...)
	}
	return dirs
}

func dirNames(dir string, match func(basename string) bool) []string {
	f, err := os.Open(dir)
	if err != nil {
		return nil
	}
	defer f.Close()

	var dirs []string
	for {
		names, _ := f.Readdirnames(100)
		if len(names) == 0 {
			break
		}
		for _, nm := range names {
			if !match(nm) {
				continue
			}

			path := filepath.Join(dir, nm)
			fi, err := os.Lstat(path)
			if err == nil && fi.IsDir() {
				dirs = append(dirs, path)
			}
		}
	}

	return dirs
}
