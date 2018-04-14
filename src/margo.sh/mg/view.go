package mg

import (
	"bytes"
	"encoding/base64"
	"golang.org/x/crypto/blake2b"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"unicode/utf8"
)

type View struct {
	Path  string
	Wd    string
	Name  string
	Hash  string
	Src   []byte
	Pos   int
	Row   int
	Col   int
	Dirty bool
	Ext   string
	Lang  string

	changed int
	kvs     KVStore
}

func newView(kvs KVStore) *View {
	return &View{kvs: kvs}
}

func (v *View) Copy(updaters ...func(*View)) *View {
	x := *v
	for _, f := range updaters {
		f(&x)
	}
	return &x
}

func (v *View) LangIs(names ...string) bool {
	for _, s := range names {
		if s == v.Lang {
			return true
		}
		if v.Ext != "" && v.Ext[1:] == s {
			return true
		}
	}
	return false
}

func (v *View) Dir() string {
	if v.Path != "" {
		return filepath.Dir(v.Path)
	}
	return v.Wd
}

func (v *View) Filename() string {
	if v.Path != "" {
		return v.Path
	}
	return filepath.Join(v.Wd, v.Name)
}

func (v *View) key() interface{} {
	type Key struct{ Hash string }
	return Key{v.Hash}
}

func (v *View) src() (src []byte, ok bool) {
	src = v.Src
	if len(src) != 0 {
		return src, true
	}

	if v.kvs != nil {
		src, _ = v.kvs.Get(v.key()).([]byte)
	}

	if v.Path == "" || v.Dirty || len(src) != 0 {
		return src, true
	}

	return nil, false
}

func (v *View) ReadAll() ([]byte, error) {
	if src, ok := v.src(); ok {
		return src, nil
	}

	r, err := v.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()

	src, err := ioutil.ReadAll(r)
	if err == nil && v.kvs != nil {
		v.kvs.Put(v.key(), src)
	}

	return src, err
}

func (v *View) Valid() bool {
	return v.Name != ""
}

func (v *View) Open() (r io.ReadCloser, err error) {
	if src, ok := v.src(); ok {
		return ioutil.NopCloser(bytes.NewReader(src)), nil
	}

	if v.Path == "" {
		return nil, os.ErrNotExist
	}

	return os.Open(v.Path)
}

func (v *View) initSrcPos() {
	src, err := v.ReadAll()
	if err != nil {
		return
	}

	v.Src = src
	v.Pos = BytePos(src, v.Pos)
	v.Hash = SrcHash(src)
	v.kvs.Put(v.key(), src)
}

func (v *View) SetSrc(s []byte) *View {
	return v.Copy(func(v *View) {
		v.Pos = 0
		v.Row = 0
		v.Col = 0
		v.Src = s
		v.Hash = SrcHash(s)
		v.Dirty = true
		v.changed++
	})
}

func SrcHash(s []byte) string {
	hash := blake2b.Sum512(s)
	return "hash:blake2b/Sum512;base64url," + base64.URLEncoding.EncodeToString(hash[:])
}

func BytePos(src []byte, charPos int) int {
	for i, c := range src {
		if !utf8.RuneStart(c) {
			continue
		}
		charPos--
		if charPos < 0 {
			return i
		}
	}
	return len(src)
}
