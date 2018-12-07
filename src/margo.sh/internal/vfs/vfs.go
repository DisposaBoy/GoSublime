package vfs

import (
	"errors"
	"fmt"
	"github.com/karrick/godirwalk"
	"io"
	"margo.sh/mg"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrPathIsNotAbsolute = errors.New("path is not absolute")

	Root = New(Options{
		Expires: func(*Node) time.Time {
			return time.Now().Add(30 * time.Second)
		},
	})
)

type Dirent = godirwalk.Dirent

type Options struct {
	Expires func(nd *Node) time.Time
}

type ScanOptions struct {
	Filter   func(de *Dirent) bool
	Dirs     func(path string, nd *Node)
	MaxDepth int
	scratch  []byte
}

type fileInfo struct {
	os.FileInfo
	expires int64
}

func (fi *fileInfo) setExpires(t time.Time) {
	fi.expires = t.UnixNano()
}

func (fi *fileInfo) expired() bool {
	if fi.expires <= 0 {
		return false
	}
	return time.Now().UnixNano() >= fi.expires
}

type dirents struct {
	path string
	ents []*Dirent
}

type FS struct{ Node }

func New(o Options) *FS {
	fs := &FS{}
	fs.opts = &o
	return fs
}

func (fs *FS) Peek(path string) *Node { return fs.peek(PathComponents(path)) }

func (fs *FS) Poke(path string, mode os.FileMode) *Node { return fs.poke(PathComponents(path)) }

func (fs *FS) Remove(path string) { fs.Peek(path).Remove() }

func (fs *FS) Stat(path string) (os.FileInfo, error) { return fs.Poke(path, 0).Stat() }

func (fs *FS) StatKV(path string) (mg.KVStore, os.FileInfo, error) { return fs.Poke(path, 0).StatKV() }

func (fs *FS) Scan(path string, so ScanOptions) error {
	if !filepath.IsAbs(path) {
		return ErrPathIsNotAbsolute
	}
	so.scratch = make([]byte, godirwalk.DefaultScratchBufferSize)
	if so.Filter == nil {
		so.Filter = DefaultScanFilter
	}
	fs.Poke(path, os.ModeDir).scan(path, so)
	return nil
}

type NodeList []*Node

func (nl NodeList) Len() int           { return len(nl) }
func (nl NodeList) Less(i, j int) bool { return nl[i].Name() < nl[j].Name() }
func (nl NodeList) Swap(i, j int)      { nl[i], nl[j] = nl[j], nl[i] }

func (nl NodeList) Copy() NodeList { return append(NodeList(nil), nl...) }

func (nl NodeList) Child(name string) *Node {
	_, c := nl.Find(name)
	return c
}

func (nl NodeList) Find(name string) (index int, c *Node) {
	for i, c := range nl {
		if c.name == name {
			return i, c
		}
	}
	return -1, nil
}

func (nl NodeList) Index(nd *Node) (index int) {
	for i, c := range nl {
		if c == nd {
			return i
		}
	}
	return -1
}

func (nl NodeList) Remove(nd *Node) {
	i := nl.Index(nd)
	if i < 0 {
		return
	}
	nl[i] = nl[len(nl)-1]
	nl[len(nl)-1] = nil
	nl = nl[:len(nl)-1]
}

type Node struct {
	parent *Node
	opts   *Options
	name   string

	mu sync.RWMutex
	fi *fileInfo
	kv *mg.KVMap
	cl NodeList
}

func (nd *Node) SomePrefix(pfx string) bool {
	return nd.Some(func(nd *Node) bool { return strings.HasPrefix(nd.name, pfx) })
}

func (nd *Node) SomeSuffix(sfx string) bool {
	return nd.Some(func(nd *Node) bool { return strings.HasSuffix(nd.name, sfx) })
}

func (nd *Node) Some(f func(nd *Node) bool) bool {
	if nd == nil {
		return false
	}

	nd.mu.RLock()
	defer nd.mu.RUnlock()

	for _, c := range nd.cl {
		if f(c) {
			return true
		}
	}
	return false
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
	if nd != nil {
		return nd.name
	}
	return ""
}

func (nd *Node) IsLeaf() bool {
	if nd != nil {
		return !nd.IsBranch()
	}
	return false
}

func (nd *Node) IsBranch() bool {
	if nd == nil {
		return false
	}

	nd.mu.RLock()
	defer nd.mu.RUnlock()

	return len(nd.cl) != 0
}

func (nd *Node) Parent() *Node {
	if nd != nil {
		return nd.parent
	}
	return nil
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

func (nd *Node) Branches(f func(path string, nd *Node)) {
	nd.branches("", f)
}

func (nd *Node) scanEnts(dl []*Dirent) (dirs []*Node) {
	cl := make([]*Node, 0, len(dl))
	for _, de := range dl {
		name, mode := de.Name(), de.ModeType()
		c := nd.cl.Child(name)
		if c == nil || !nd.modeEq(mode) {
			c = nd.mkNode(name, nil)
		}
		cl = append(cl, c)
		if de.IsDir() {
			dirs = append(dirs, c)
		}
	}
	nd.cl = cl
	return dirs
}

func (nd *Node) readDir(root string, so ScanOptions) []*Dirent {
	l, _ := godirwalk.ReadDirents(root, so.scratch)
	if so.Filter == nil || len(l) == 0 {
		return l
	}
	ents := l[:0]
	for _, de := range l {
		if so.Filter(de) {
			ents = append(ents, de)
		}
	}
	return ents
}

func (nd *Node) scan(root string, so ScanOptions) {
	if so.MaxDepth == 0 {
		return
	}
	so.MaxDepth--

	ents := nd.readDir(root, so)

	nd.mu.Lock()
	dirs := nd.scanEnts(ents)
	nd.mu.Unlock()

	if so.Dirs != nil {
		so.Dirs(root, nd)
	}

	for _, c := range dirs {
		c.scan(filepath.Join(root, c.name), so)
	}
}

func (nd *Node) branches(root string, f func(path string, nd *Node)) {
	if nd == nil {
		return
	}

	cl := nd.Children()

	if len(cl) == 0 {
		return
	}

	if root == "" {
		// we're the origin node, we can't call f
		root = nd.Path()
	} else {
		f(root, nd)
	}

	for _, c := range cl {
		c.branches(filepath.Join(root, c.name), f)
	}
}

func (nd *Node) Path() string {
	if nd == nil {
		return ""
	}

	sep := string(filepath.Separator)
	pth := nd.name
	for p := nd.parent; !p.IsRoot(); p = p.parent {
		pth = p.name + sep + pth
	}
	if sep == "/" {
		pth = sep + pth
	}
	return pth
}

func (nd *Node) Remove() {
	if nd.IsRoot() {
		return
	}

	p := nd.parent
	p.mu.Lock()
	defer p.mu.Unlock()

	p.cl.Remove(nd)
}

func (nd *Node) IsRoot() bool { return nd == nil || nd.parent == nil }

func (nd *Node) peek(path []string) *Node {
	if nd == nil || len(path) == 0 {
		return nd
	}
	name, path := path[0], path[1:]
	return nd.cl.Child(name).peek(path)
}

func (nd *Node) assertPoke() {
	if nd == nil {
		panic("poke* called a nil node")
	}
}

func (nd *Node) poke(path []string) *Node {
	nd.assertPoke()

	if len(path) == 0 {
		return nd
	}
	name, path := path[0], path[1:]
	nd = nd.touch(name, nil)
	if len(path) == 0 {
		return nd
	}
	return nd.poke(path)
}

func (nd *Node) touch(name string, fi os.FileInfo) *Node {
	nd.assertPoke()

	nd.mu.Lock()
	defer nd.mu.Unlock()

	i, c := nd.cl.Find(name)
	if c != nil && (fi == nil || !c.modeEq(fi.Mode())) {
		return c
	}

	c = nd.mkNode(name, fi)
	if i >= 0 {
		nd.cl[i] = c
	} else {
		nd.cl = append(nd.cl, c)
	}
	return c
}

func (nd *Node) modeEq(p os.FileMode) bool {
	q := nd.mode()
	switch {
	case p == 0, q == 0, p == q:
		return true
	case p.IsDir() == q.IsDir():
		return true
	case p&os.ModeSymlink != 0:
		return true
	}
	return false
}

func (nd *Node) mode() os.FileMode {
	switch {
	case nd == nil:
		return 0
	case nd.fi != nil:
		return nd.fi.Mode()
	case len(nd.cl) != 0:
		return os.ModeDir
	}
	return 0
}

func (nd *Node) mkNode(name string, fi os.FileInfo) *Node {
	return &Node{
		parent: nd,
		opts:   nd.opts,
		name:   name,
	}
}

func (nd *Node) Children() NodeList {
	if nd == nil {
		return nil
	}

	nd.mu.Lock()
	defer nd.mu.Unlock()

	return nd.cl.Copy()
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
	if nd.IsRoot() && len(cl) == 1 && cl[0].name == "" {
		cl = cl[0].Children()
	}
	sort.Sort(cl)

	type C struct {
		*Node
		s string
	}
	l := make([]C, 0, len(cl))
	for _, c := range cl {
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

func (nd *Node) StatKV() (mg.KVStore, os.FileInfo, error) {
	if nd == nil {
		return nil, nil, os.ErrNotExist
	}

	nd.mu.Lock()
	defer nd.mu.Unlock()

	fi, err := nd.stat()
	if err != nil {
		return nil, nil, err
	}
	if nd.kv == nil {
		nd.kv = &mg.KVMap{}
	}
	return nd.kv, fi, nil
}

func (nd *Node) Stat() (os.FileInfo, error) {
	if nd == nil {
		return nil, os.ErrNotExist
	}

	nd.mu.Lock()
	defer nd.mu.Unlock()

	return nd.stat()
}

func (nd *Node) stat() (os.FileInfo, error) {
	if fi := nd.fi; fi != nil && !fi.expired() {
		return fi.FileInfo, nil
	}

	fi, err := os.Stat(nd.Path())
	// only reset if the mtime changed
	reset := fi == nil || nd.fi == nil || !nd.fi.ModTime().Equal(fi.ModTime())
	if reset && nd.kv != nil {
		nd.kv.Clear()
	}

	if err != nil {
		nd.fi = nil
		nd.cl = nil
		return nil, err
	}

	if reset && fi.IsDir() {
		nd.scanEnts(nd.readDir(nd.Path(), ScanOptions{}))
	}

	nd.fi = &fileInfo{FileInfo: fi}
	if exp := nd.opts.Expires; exp != nil {
		nd.fi.setExpires(exp(nd))
	}
	return fi, nil
}

func PathComponents(p string) []string {
	p = filepath.ToSlash(p)
	p = path.Clean(p)
	if filepath.Separator == '/' {
		p = strings.TrimPrefix(p, "/")
	}
	return strings.Split(p, "/")
}

func DefaultScanFilter(de *Dirent) bool {
	nm := de.Name()
	return nm != "" && nm[0] != '.' && nm[0] != '_'
}
