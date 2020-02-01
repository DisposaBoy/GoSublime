package kimporter

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"margo.sh/mg"
	"path/filepath"
	"sync"
)

type kpFile struct {
	CheckFuncs bool
	Mx         *mg.Ctx
	Fset       *token.FileSet
	Fn         string
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
	// TODO: try to patch up some of the broken files
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

func parseDir(mx *mg.Ctx, bcx *build.Context, fset *token.FileSet, dir string, srcMap map[string][]byte, ks *state) (*build.Package, []*kpFile, []*ast.File, error) {
	defer mx.Profile.Push(`Kim-Porter: parseDir(` + dir + `)`).Pop()

	bp, err := bcx.ImportDir(dir, 0)
	if err != nil {
		return nil, nil, nil, err
	}
	wg := sync.WaitGroup{}
	testFiles := bp.TestGoFiles
	if !ks.Tests {
		testFiles = nil
	}
	kpFiles := make([]*kpFile, 0, len(bp.GoFiles)+len(bp.CgoFiles)+len(testFiles))
	for _, l := range [][]string{bp.GoFiles, bp.CgoFiles, testFiles} {
		for _, nm := range l {
			fn := filepath.Join(dir, nm)
			kf := &kpFile{
				Mx:         mx,
				Fset:       fset,
				Fn:         fn,
				Src:        srcMap[fn],
				CheckFuncs: ks.CheckFuncs,
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

	astFiles := make([]*ast.File, 0, len(kpFiles))
	for _, kf := range kpFiles {
		if kf.File != nil {
			astFiles = append(astFiles, kf.File)
		}
		if err == nil && kf.Err != nil {
			err = kf.Err
		}
	}
	return bp, kpFiles, astFiles, err
}
