package vfs

import (
	"fmt"
	"github.com/karrick/godirwalk"
	"io"
	"margo.sh/mgutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	asyncC = make(chan func(), 1000)
)

func init() {
	go func() {
		for f := range asyncC {
			f()
		}
	}()
}

func async(f func()) {
	select {
	case asyncC <- f:
	default:
		go f()
	}
}

// TODO: add .Trim() support to allow periodically removing unused leaf nodes to reduce memory.

type ScanOptions struct {
	Filter   func(de *Dirent) bool
	Dirs     func(nd *Node)
	MaxDepth int

	scratch []byte
}

type FS struct{ Node }

func New() *FS {
	return &FS{}
}

func (fs *FS) Invalidate(path string) { fs.Peek(path).Invalidate() }

func (fs *FS) Stat(path string) (*Node, os.FileInfo, error) {
	nd := fs.Poke(path)
	fi, err := nd.Stat()
	return nd, fi, err
}

func (fs *FS) ReadDir(path string) ([]os.FileInfo, error) { return fs.Poke(path).ReadDir() }

func (fs *FS) IsDir(path string) bool { return fs.Poke(path).IsDir() }

func (fs *FS) Memo(path string) (*Node, *mgutil.Memo, error) {
	nd := fs.Poke(path)
	m, err := nd.Memo()
	return nd, m, err
}

func (fs *FS) Scan(path string, so ScanOptions) {
	so.scratch = make([]byte, godirwalk.DefaultScratchBufferSize)
	fs.Poke(path).scan(path, &so, 0)
}

type Node struct {
	parent *Node
	name   string

	mu sync.Mutex
	cl *NodeList
	mt *meta
}

func (nd *Node) String() string {
	switch {
	case nd == nil:
		return ""
	case nd.IsBranch():
		return nd.name + "/"
	}
	return nd.name
}

func (nd *Node) Name() string {
	if nd == nil {
		return ""
	}
	return nd.name
}

func (nd *Node) IsLeaf() bool {
	// if nd is nil, it's neither a branch nor a leaf
	if nd == nil {
		return false
	}
	return !nd.IsBranch()
}

func (nd *Node) IsBranch() bool {
	if nd == nil {
		return false
	}

	nd.mu.Lock()
	defer nd.mu.Unlock()

	return nd.isBranch()
}

func (nd *Node) isBranch() bool { return nd.cl.Len() != 0 }

func (nd *Node) Parent() *Node {
	if nd == nil {
		return nil
	}
	return nd.parent
}

func (nd *Node) IsDescendant(ancestor *Node) bool {
	if nd == nil || ancestor == nil {
		return false
	}
	for p := nd.parent; !p.IsRoot(); p = p.parent {
		if p == ancestor {
			return true
		}
	}
	return false
}

func (nd *Node) scanEnts(so *ScanOptions, dl []*godirwalk.Dirent) (dirs []*Node) {
	finalize := func(nd *Node, de *godirwalk.Dirent) {
		mt := nd.meta()
		mt.resetInfo(de.ModeType(), time.Time{})
		if so.Dirs != nil && mt.fmode.IsDir() {
			dirs = append(dirs, nd)
		}
	}
	cl := make([]*Node, 0, len(dl))
	for _, de := range dl {
		c := nd.cl.Node(de.Name())
		if c == nil {
			c = nd.mkNode(de.Name())
			finalize(c, de)
		} else {
			c.mu.Lock()
			finalize(c, de)
			c.mu.Unlock()
		}
		cl = append(cl, c)
	}
	nd.cl = &NodeList{l: cl}
	return dirs
}

func (nd *Node) readDirents(root string, so *ScanOptions) []*godirwalk.Dirent {
	l, _ := godirwalk.ReadDirents(root, so.scratch)
	if so.Filter == nil || len(l) == 0 {
		return l
	}
	ents := l[:0]
	for _, de := range l {
		if so.Filter(&Dirent{name: de.Name(), fmode: fmode(de.ModeType())}) {
			ents = append(ents, de)
		}
	}
	return ents
}

func (nd *Node) scan(root string, so *ScanOptions, depth int) {
	ents := nd.readDirents(root, so)

	nd.mu.Lock()
	dirs := nd.scanEnts(so, ents)
	nd.mu.Unlock()

	if so.Dirs != nil {
		so.Dirs(nd)
	}

	depth++
	if so.MaxDepth > 0 && depth >= so.MaxDepth {
		return
	}
	root += string(filepath.Separator)
	for _, c := range dirs {
		c.scan(root+c.name, so, depth)
	}
}

func (nd *Node) Branches(f func(nd *Node)) {
	if nd == nil {
		return
	}

	cl := nd.Children()
	if cl.Len() == 0 {
		return
	}

	f(nd)
	for _, c := range cl.Nodes() {
		c.Branches(f)
	}
}

func (nd *Node) Path() string {
	str := strings.Builder{}
	var walk func(*Node, int)
	walk = func(nd *Node, n int) {
		if nd.IsRoot() {
			str.Grow(n)
			return
		}
		walk(nd.parent, n+1+len(nd.name))
		str.WriteByte(filepath.Separator)
		str.WriteString(nd.name)
	}
	walk(nd, 0)

	if str.Len() == 0 {
		if filepath.Separator == '/' {
			return "/"
		}
		return ""
	}

	pth := str.String()
	if c := byte(filepath.Separator); c != '/' && pth[0] == c {
		pth = pth[1:]
	}
	return pth
}

func (nd *Node) IsRoot() bool { return nd == nil || nd.parent == nil }

func (nd *Node) Peek(path string) *Node {
	if nd == nil {
		return nil
	}
	name, path := splitPath(path)
	if name == "" {
		return nd
	}

	nd.mu.Lock()
	c := nd.cl.Node(name)
	nd.mu.Unlock()

	return c.Peek(path)
}

func (nd *Node) Poke(path string) *Node {
	if nd == nil {
		panic("Poke() called on a nil Node")
	}
	name, rest := splitPath(path)
	if name == "" {
		return nd
	}
	return nd.touch(name).Poke(rest)
}

func (nd *Node) touch(name string) *Node {
	nd.mu.Lock()
	defer nd.mu.Unlock()

	if c := nd.cl.Node(name); c != nil {
		return c
	}

	c := nd.mkNode(name)
	nd.cl = nd.cl.Add(c)
	return c
}

func (nd *Node) mkNode(name string) *Node {
	return &Node{parent: nd, name: name}
}

func (nd *Node) meta() *meta {
	if nd.mt == nil {
		nd.mt = &meta{}
	}
	return nd.mt
}

func (nd *Node) Ls() *NodeList {
	if nd == nil {
		return nil
	}

	nd.mu.Lock()
	defer nd.mu.Unlock()

	nd.sync()
	return nd.cl
}

func (nd *Node) Children() *NodeList {
	if nd == nil {
		return nil
	}

	nd.mu.Lock()
	defer nd.mu.Unlock()

	return nd.cl
}

func (nd *Node) Print(w io.Writer) {
	nd.PrintWithFilter(w, func(nd *Node) string { return nd.String() })
}

func (nd *Node) PrintWithFilter(w io.Writer, filter func(*Node) string) {
	nd.print(w, filter, "")
}

func (nd *Node) print(w io.Writer, filter func(*Node) string, indent string) {
	if nd == nil {
		return
	}

	midPfx := indent + "├─"
	endPfx := indent + "└─"
	midInd := indent + "│ "
	endInd := indent + "  "

	cl := nd.Children()
	if nd.IsRoot() && cl.Len() == 1 {
		if nl := cl.Nodes(); nl[0].name == "" {
			cl = nl[0].Children()
		}
	}
	cl = cl.Sorted()

	type C struct {
		*Node
		s string
	}
	l := make([]C, 0, cl.Len())
	for _, c := range cl.Nodes() {
		if s := filter(c); s != "" {
			l = append(l, C{c, s})
		}
	}
	for i, c := range l {
		pfx, ind := midPfx, midInd
		if i == len(l)-1 {
			pfx, ind = endPfx, endInd
		}
		fmt.Fprintf(w, "%s %s\n", pfx, c.s)
		c.print(w, filter, ind)
	}
}

func (nd *Node) Memo() (*mgutil.Memo, error) {
	if nd == nil {
		return nil, os.ErrNotExist
	}

	nd.mu.Lock()
	defer nd.mu.Unlock()

	mt, err := nd.sync()
	if err != nil {
		return nil, err
	}
	return mt.memo(), nil
}

func (nd *Node) Invalidate() {
	if nd == nil {
		return
	}

	nd.mu.Lock()
	defer nd.mu.Unlock()

	nd.mt.invalidate()
}

func (nd *Node) Stat() (os.FileInfo, error) {
	if nd == nil {
		return nil, os.ErrNotExist
	}

	nd.mu.Lock()
	defer nd.mu.Unlock()

	mt, err := nd.sync()
	if err != nil {
		return nil, err
	}
	fi := &FileInfo{Node: nd, fmode: mt.fmode}
	if !fi.fmode.IsValid() && nd.cl.Len() != 0 {
		fi.fmode = fmodeDir
	}
	return fi, nil
}

func (nd *Node) sync() (*meta, error) {
	mt := nd.meta()
	if mt.ok() {
		return mt, nil
	}
	path := nd.Path()
	fi, err := os.Stat(path)
	reset := fi == nil || !mt.fmode.IsValid() || mt.modts != tsTime(fi.ModTime())
	if reset {
		mt.resetMemo()
	}
	// if a file in a directory changed, the dir's memo is cleared as well because
	// a dir's memo is primarily used to store pkg/dir data that depends on the file
	if reset && !nd.isBranch() {
		nd.resetParent()
	}
	if err != nil {
		mt.invalidate()
		nd.cl = nil
		return nil, err
	}
	mt.resetInfo(fi.Mode(), fi.ModTime())
	if reset && fi.IsDir() {
		so := &ScanOptions{MaxDepth: 1}
		nd.scanEnts(so, nd.readDirents(path, so))
	}
	return mt, nil
}

func (nd *Node) resetParent() {
	p := nd.Parent()
	if p == nil {
		return
	}
	ts := tsNow()
	async(func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.mt.resetMemoAfter(ts)
	})
}

func (nd *Node) IsDir() bool {
	if nd == nil {
		return false
	}

	nd.mu.Lock()
	defer nd.mu.Unlock()

	mt, err := nd.sync()
	return err == nil && mt.fmode.IsDir()
}

func (nd *Node) ReadDir() ([]os.FileInfo, error) {
	if nd == nil {
		return nil, os.ErrNotExist
	}

	nd.mu.Lock()
	_, err := nd.sync()
	cl := nd.cl
	nd.mu.Unlock()

	if err != nil {
		return nil, err
	}

	l := make([]os.FileInfo, 0, cl.Len())
	for _, c := range cl.Nodes() {
		if fi, err := c.Stat(); err == nil {
			l = append(l, fi)
		}
	}
	return l, nil
}

func (nd *Node) Locate(name string) (*Node, os.FileInfo, error) {
	c := nd.touch(name)
	if fi, err := c.Stat(); err == nil {
		return c, fi, err
	}
	if nd.IsRoot() {
		return nil, nil, os.ErrNotExist
	}
	return nd.parent.Locate(name)
}

func isSep(r byte) bool { return r == '/' || r == '\\' }

func splitPath(p string) (head, tail string) {
	for p != "" && isSep(p[0]) {
		p = p[1:]
	}
	for i := 0; i < len(p); i++ {
		if isSep(p[i]) {
			return p[:i], p[i:]
		}
	}
	return p, ""
}
