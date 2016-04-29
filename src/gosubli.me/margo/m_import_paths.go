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
	"sync"
	"time"
)

var (
	qidCache = struct {
		sync.Mutex
		m map[string]*qidNode
	}{m: map[string]*qidNode{}}
)

type qidNode struct {
	sync.Mutex
	Pkg     *build.Package
	EntName string
	DirMod  time.Time
	EntMod  time.Time
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

	srcImportPath := quickImportPath(srcDir)

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

func quickImportPath(srcDir string) string {
	dir := filepath.ToSlash(filepath.Clean(srcDir))
	if i := strings.LastIndex(dir, "/src/"); i >= 0 {
		return dir[i+5:]
	}
	return ""
}

func quickImportDir(bctx *build.Context, rootDirs []string, srcDir string) *build.Package {
	srcDir = filepath.Clean(srcDir)
	qidCache.Lock()
	qn := qidCache.m[srcDir]
	if qn == nil {
		qn = &qidNode{
			Pkg: &build.Package{
				Dir:        srcDir,
				ImportPath: quickImportPath(srcDir),
			},
		}
		qidCache.m[srcDir] = qn
	}
	qidCache.Unlock()

	qn.Lock()
	defer qn.Unlock()

	if qn.Pkg.ImportPath == "" {
		return nil
	}

	dirMod := fileModTime(srcDir)
	if dirMod.IsZero() {
		return nil
	}

	if qn.DirMod.Equal(dirMod) {
		if qn.Pkg.Name == "" {
			// not a Go pkg
			return nil
		}
		if qn.EntName != "" && qn.EntMod.Equal(fileModTime(filepath.Join(srcDir, qn.EntName))) {
			return qn.Pkg
		}
	}

	// reset cache
	qn.DirMod = dirMod
	qn.EntName = ""
	qn.Pkg.Name = ""

	f, err := os.Open(srcDir)
	if err != nil {
		return nil
	}
	defer f.Close()

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
			entMod := fileModTime(path)
			if entMod.IsZero() {
				continue
			}

			mode := parser.PackageClauseOnly | parser.ParseComments
			af, _ := parser.ParseFile(fset, path, nil, mode)
			qn.Pkg.Name = astFileName(af)
			if qn.Pkg.Name != "" {
				qn.EntName = nm
				qn.EntMod = entMod
				return qn.Pkg
			}
		}
	}
	return nil
}

func fileModTime(fn string) time.Time {
	if fi := fileInfo(fn); fi != nil {
		return fi.ModTime()
	}
	return time.Time{}
}

func fileInfo(fn string) os.FileInfo {
	fi, _ := os.Lstat(fn)
	return fi
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
			fi := fileInfo(path)
			if fi != nil && fi.IsDir() {
				dirs = append(dirs, path)
			}
		}
	}

	return dirs
}
