package kimporter

import (
	"go/build"
	"margo.sh/mg"
	"os"
	"testing"
	"time"
)

func init() {
	build.Default.GOPATH = "/user/gp/"
	os.Setenv("GOPATH", build.Default.GOPATH)
}

func Test(t *testing.T) {
	mx := mg.NewTestingAgent(nil, nil, os.Stderr).Store.NewCtx(nil)
	mx.Log.Dbg.SetFlags(0)
	mx.Log.Dbg.SetPrefix("")

	ipath := "."
	srcDirs := []string{
		"/user/gp/src/github.com/faiface/pixel/pixelgl/",
		"/krku/src/oya.to/fabricant/",
	}
	for _, srcDir := range srcDirs {
		for i := 0; i < 1; i++ {
			kp := New(mx, &Config{Tests: true, NoConcurrency: false})
			start := time.Now()
			pkg, err := kp.ImportFrom(ipath, srcDir, 0)
			mx.Log.Dbg.Println("pk:", srcDir, "\n>  ", pkg, "\n>  ", time.Since(start), pkg != nil && pkg.Complete(), err)
		}
	}
}
