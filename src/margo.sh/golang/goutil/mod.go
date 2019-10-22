package goutil

import (
	"margo.sh/mg"
	"margo.sh/mgutil"
	"margo.sh/vfs"
	"path/filepath"
)

const (
	ModEnvVar = "GO111MODULE"
)

// ModEnabled returns true of Go modules are enabled in srcDir
func ModEnabled(mx *mg.Ctx, srcDir string) bool {
	// - If on Go <= go1.12 and inside GOPATH — defaults to old 1.10 behavior (ignoring modules)
	// - Outside GOPATH while inside a file tree with a go.mod — defaults to modules behavior
	// - GO111MODULE environment variable:
	//     unset or auto — default behavior above
	//     on — force module support on regardless of directory location
	//     off — force module support off regardless of directory location

	switch mx.Env.Getenv(ModEnvVar, "") {
	case "on":
		return true
	case "off":
		return false
	}

	bctx := BuildContext(mx)
	type K struct{ SrcDirKey }
	k := K{MakeSrcDirKey(bctx, srcDir)}
	if v, ok := mx.Get(k).(bool); ok {
		return v
	}

	if v := Version; v.Major <= 1 && v.Minor <= 12 {
		for _, gp := range PathList(bctx.GOPATH) {
			p := filepath.Join(gp, "src")
			if mgutil.IsParentDir(p, k.SrcDir) || k.SrcDir == p {
				mx.Put(k, false)
				return false
			}
		}
	}

	modFileExists := ModFileNd(mx, k.SrcDir) != nil
	mx.Put(k, modFileExists)
	return modFileExists
}

func ModFileNd(mx *mg.Ctx, srcDir string) *vfs.Node {
	bctx := BuildContext(mx)
	type K struct{ SrcDirKey }
	k := K{MakeSrcDirKey(bctx, srcDir)}
	if v, ok := mx.Get(k).(*vfs.Node); ok {
		return v
	}
	nd, _, _ := mx.VFS.Poke(k.SrcDir).Locate("go.mod")
	mx.Put(k, nd)
	return nd
}
