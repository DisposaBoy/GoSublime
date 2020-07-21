package kimporter

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"margo.sh/golang/gopkg"
	"margo.sh/mg"
	"margo.sh/vfs"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type kpFile struct {
	CheckFuncs bool
	Mx         *mg.Ctx
	Fset       *token.FileSet
	Fn         string
	Nm         string
	Src        []byte
	Err        error
	*ast.File
}

func (kf *kpFile) init() {
	if len(kf.Src) == 0 {
		kf.Src, kf.Err = kf.Mx.VFS.ReadBlob(kf.Fn).ReadFile()
		if kf.Err != nil {
			return
		}
	}
	kf.File, kf.Err = parser.ParseFile(kf.Fset, kf.Fn, kf.Src, 0)
	if kf.File == nil {
		return
	}
	if !kf.CheckFuncs {
		// trim func bodies to reduce memory
		for _, d := range kf.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				d.Body = &ast.BlockStmt{}
			}
		}
	}
}

func bldImportDir(bcx *build.Context, pp *gopkg.PkgPath, pkgSrc map[string][]byte) (*build.Package, error) {
	if len(pkgSrc) == 0 {
		bp, err := bcx.ImportDir(pp.Dir, 0)
		if err != nil {
			return nil, err
		}
		return bp, nil
	}
	bp := &build.Package{
		ImportPath: pp.ImportPath,
		Dir:        pp.Dir,
	}
	for fn, _ := range pkgSrc {
		switch {
		case !strings.HasSuffix(fn, ".go"):
			continue
		case strings.HasSuffix(fn, "_test.go"):
			bp.TestGoFiles = append(bp.TestGoFiles, fn)
		default:
			bp.GoFiles = append(bp.GoFiles, fn)
		}
	}
	fset := token.NewFileSet()
	importsList := func(fns []string) []string {
		l := []string{}
		for _, fn := range fns {
			af, _ := parser.ParseFile(fset, fn, pkgSrc[fn], parser.ImportsOnly)
			if af == nil {
				continue
			}
			for _, imp := range af.Imports {
				if imp.Path == nil {
					continue
				}
				s, err := strconv.Unquote(imp.Path.Value)
				if err == nil && s != "" {
					l = append(l, s)
				}
			}
		}
		return l
	}
	bp.Imports = importsList(bp.GoFiles)
	bp.TestImports = importsList(bp.TestGoFiles)
	return bp, nil
}

func parseDir(mx *mg.Ctx, bcx *build.Context, fset *token.FileSet, pp *gopkg.PkgPath, srcMap map[string][]byte, ks *state, pkgSrc map[string][]byte) (*build.Package, map[string]*ast.File, []*ast.File, error) {
	defer mx.Profile.Push(`Kim-Porter: parseDir(` + pp.Dir + `)`).Pop()

	bp, err := bldImportDir(bcx, pp, pkgSrc)
	if err != nil {
		return nil, nil, nil, err
	}
	if !ks.Tests {
		bp.TestGoFiles = nil
	}
	kpFiles := make([]*kpFile, 0, len(bp.GoFiles)+len(bp.CgoFiles)+len(bp.TestGoFiles))
	if cap(kpFiles) == 0 {
		return nil, nil, nil, &build.NoGoError{Dir: pp.Dir}
	}
	wg := sync.WaitGroup{}
	for _, l := range [][]string{bp.GoFiles, bp.CgoFiles, bp.TestGoFiles} {
		for _, nm := range l {
			fn := nm
			if !vfs.IsViewPath(fn) && !filepath.IsAbs(fn) {
				fn = filepath.Join(pp.Dir, nm)
			}
			nm = filepath.Base(fn)
			kf := &kpFile{
				Mx:         mx,
				Fset:       fset,
				Fn:         fn,
				Nm:         nm,
				CheckFuncs: ks.CheckFuncs,
			}
			kf.Src = pkgSrc[nm]
			if kf.Src == nil {
				kf.Src = srcMap[fn]
			}
			kpFiles = append(kpFiles, kf)
			wg.Add(1)
			go func() {
				defer wg.Done()

				kf.init()
			}()
		}
	}
	wg.Wait()

	filesList := make([]*ast.File, 0, len(kpFiles))
	filesMap := make(map[string]*ast.File, len(kpFiles))
	for _, kf := range kpFiles {
		if kf.File != nil {
			filesList = append(filesList, kf.File)
			filesMap[kf.Nm] = kf.File
		}
		if err == nil && kf.Err != nil {
			err = kf.Err
		}
	}
	return bp, filesMap, filesList, err
}
