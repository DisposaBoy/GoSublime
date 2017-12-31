package importpaths

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"gosublime/margo"
	"os"
	"path/filepath"
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

func PathFilter(path string) bool {
	return margo.FilterPath(path) &&
		margo.FilterPathExt(path) &&
		!strings.Contains(filepath.Base(path), "node_modules")
}

func MakeImportPathsFunc(pathFilter margo.PathFilterFunc) margo.ImportPathsFunc {
	return func(srcDir string, bctx *build.Context) map[string]string {
		return ImportPaths(srcDir, bctx, pathFilter)
	}
}

func ImportPaths(srcDir string, bctx *build.Context, pathFilter margo.PathFilterFunc) map[string]string {
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
		pkgs = append(pkgs, importablePackages(dir, importDir, pathFilter)...)
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
			if af == nil {
				continue
			}

			if ok, _ := bctx.MatchFile(srcDir, nm); !ok {
				continue
			}

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
				ln = strings.TrimSpace(ln)
				if strings.HasPrefix(ln, "+build ") && strings.Index(ln+" ", " ignore ") > 0 {
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

func importablePackages(root string, importDir func(path string) *build.Package, pathFilter margo.PathFilterFunc) []*build.Package {
	dirs := allDirNames(root, pathFilter)
	var ents []*build.Package
	for _, dir := range dirs {
		if p := importDir(dir); p != nil {
			ents = append(ents, p)
		}
	}
	return ents
}

func allDirNames(dir string, pathFilter margo.PathFilterFunc) []string {
	dirs := dirNames(dir, pathFilter)
	for _, dir := range dirs {
		dirs = append(dirs, allDirNames(dir, pathFilter)...)
	}
	return dirs
}

func dirNames(dir string, pathFilter func(basename string) bool) []string {
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
			path := filepath.Join(dir, nm)
			if !pathFilter(path) {
				continue
			}
			fi := fileInfo(path)
			if fi != nil && fi.IsDir() {
				dirs = append(dirs, path)
			}
		}
	}

	return dirs
}
