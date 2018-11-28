package pkglst

import (
	"bytes"
	"fmt"
	"github.com/ugorji/go/codec"
	"margo.sh/mg"
	"margo.sh/mgpf"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Pkg struct {
	IsCommand     bool
	ImportablePfx string

	// The following fields are a subset of build.Package
	Dir        string
	Name       string
	ImportPath string
	Standard   bool
}

var (
	internalSepDir = filepath.FromSlash("/internal/")
	vendorSepDir   = filepath.FromSlash("/vendor/")
)

func (p *Pkg) Importable(srcDir string) bool {
	if p.IsCommand {
		return false
	}
	if p.Dir == srcDir {
		return false
	}
	if s := p.ImportablePfx; s != "" {
		return strings.HasPrefix(srcDir, s) || srcDir == s[:len(s)-1]
	}
	return true
}

func (p *Pkg) dirPfx(dir, slash string) string {
	if i := strings.LastIndex(dir, slash); i >= 0 {
		return filepath.Dir(dir[:i+len(slash)-1]) + string(filepath.Separator)
	}
	if d := strings.TrimSuffix(dir, slash[:len(slash)-1]); d != dir {
		return filepath.Dir(d) + string(filepath.Separator)
	}
	return ""
}

func (p *Pkg) finalize() {
	p.Dir = filepath.Clean(p.Dir)
	p.IsCommand = p.Name == "main"

	// does importing from the 'vendor' and 'internal' dirs work the same?
	// who cares... I'm the supreme, I make the rules in this outpost...
	p.ImportablePfx = p.dirPfx(p.Dir, internalSepDir)
	if p.ImportablePfx == "" {
		p.ImportablePfx = p.dirPfx(p.Dir, vendorSepDir)
	}

	s := p.ImportPath
	switch i := strings.LastIndex(s, "/vendor/"); {
	case i >= 0:
		p.ImportPath = s[i+len("/vendor/"):]
	case strings.HasPrefix(s, "vendor/"):
		p.ImportPath = s[len("vendor/"):]
	}
}

type View struct {
	List         []*Pkg
	ByDir        map[string]*Pkg
	ByImportPath map[string][]*Pkg
	ByName       map[string][]*Pkg
}

func (vu View) shallowClone(lstLen int) View {
	x := View{
		ByDir:        make(map[string]*Pkg, len(vu.ByDir)+lstLen),
		ByImportPath: make(map[string][]*Pkg, len(vu.ByImportPath)+lstLen),
		ByName:       make(map[string][]*Pkg, len(vu.ByName)+lstLen),
	}
	for k, p := range vu.ByDir {
		x.ByDir[k] = p
	}
	for k, l := range vu.ByImportPath {
		x.ByImportPath[k] = l[:len(l):len(l)]
	}
	for k, l := range vu.ByName {
		x.ByName[k] = l[:len(l):len(l)]
	}
	return x
}

func (vu View) PruneDir(dir string) View {
	dir = filepath.Clean(dir)
	p, exists := vu.ByDir[dir]
	if !exists {
		return vu
	}

	delpkg := func(m map[string][]*Pkg, k string) {
		l := m[k]
		if len(l) == 0 {
			return
		}

		x := make([]*Pkg, 0, len(l)-1)
		for _, p := range l {
			if p.Dir != dir {
				x = append(x, p)
			}
		}

		if len(x) == 0 {
			delete(m, k)
		} else {
			m[k] = x
		}
	}
	x := vu.shallowClone(0)
	delete(x.ByDir, p.Dir)
	delpkg(x.ByImportPath, p.ImportPath)
	delpkg(x.ByName, p.Name)
	return x
}

func (vu View) Add(lst ...*Pkg) View {
	x := vu.shallowClone(len(lst))

	for _, p := range lst {
		x.ByDir[p.Dir] = p
		x.ByImportPath[p.ImportPath] = append(x.ByImportPath[p.ImportPath], p)
		x.ByName[p.Name] = append(x.ByName[p.Name], p)
	}

	x.List = make([]*Pkg, 0, len(x.ByDir))
	for _, p := range x.ByDir {
		x.List = append(x.List, p)
	}
	sort.Slice(x.List, func(i, j int) bool {
		a, b := x.List[i], x.List[j]
		switch {
		case a.Name != b.Name:
			return a.Name < b.Name
		case a.ImportPath != b.ImportPath:
			return a.ImportPath < b.ImportPath
		default:
			return a.Dir < b.Dir
		}
	})

	return x
}

type Cache struct {
	mu   sync.RWMutex
	view View
}

func (cc *Cache) Scan(mx *mg.Ctx, dir string) (output []byte, _ error) {
	lst, out, err := cc.goList(mx, dir)

	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.view = cc.view.Add(lst...)

	return out, err
}

func (cc *Cache) PruneDir(dir string) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.view = cc.view.PruneDir(dir)
}

func (cc *Cache) Add(l ...Pkg) {
	x := make([]*Pkg, len(l))
	for i, p := range l {
		p.finalize()
		x[i] = &p
	}

	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.view = cc.view.Add(x...)
}

func (cc *Cache) View() View {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	return cc.view
}

func (cc *Cache) goList(mx *mg.Ctx, dir string) (_ []*Pkg, output []byte, _ error) {
	start := time.Now()
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd := exec.Command("go", "list", "-e", "-json", "./...")
	cmd.Dir = dir
	cmd.Env = mx.Env.Merge(mg.EnvMap{
		"GOPROXY":     "off",
		"GO111MODULE": "off",
	}).Environ()
	cmd.Stdout = outBuf
	cmd.Stderr = errBuf

	err := cmd.Run()
	cmdDur := mgpf.D(time.Since(start))

	start = time.Now()
	lst := []*Pkg{}
	dec := codec.NewDecoder(outBuf, &codec.JsonHandle{})
	for {
		p := &Pkg{}
		err := dec.Decode(p)
		if p.Name != "" && p.ImportPath != "" && p.Dir != "" {
			p.finalize()
			lst = append(lst, p)
		}
		if err != nil {
			break
		}
	}
	decDur := mgpf.D(time.Since(start))

	fmt.Fprintf(errBuf, "``` packages=%d, list=%s, decode=%s, error=%v ```\n", len(lst), cmdDur, decDur, err)
	if err != nil {
		fmt.Fprintf(errBuf, "``` Error: %s```\n", err)
	}

	return lst, bytes.TrimSpace(errBuf.Bytes()), err
}
