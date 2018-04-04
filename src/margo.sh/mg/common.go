package mg

import (
	"io"
	"path/filepath"
	"strings"
	"sync"
)

type StrSet []string

func (s StrSet) Add(l ...string) StrSet {
	res := make(StrSet, 0, len(s)+len(l))
	for _, lst := range [][]string{[]string(s), l} {
		for _, p := range lst {
			if !res.Has(p) {
				res = append(res, p)
			}
		}
	}
	return res
}

func (s StrSet) Has(p string) bool {
	for _, q := range s {
		if p == q {
			return true
		}
	}
	return false
}

type EnvMap map[string]string

func (e EnvMap) Add(k, v string) EnvMap {
	m := make(EnvMap, len(e)+1)
	for k, v := range e {
		m[k] = v
	}
	m[k] = v
	return m
}

func (e EnvMap) Environ() []string {
	l := make([]string, 0, len(e))
	for k, v := range e {
		l = append(l, k+"="+v)
	}
	return l
}

func (e EnvMap) Get(k, def string) string {
	if v := e[k]; v != "" {
		return v
	}
	return def
}

func (e EnvMap) List(k string) []string {
	return strings.Split(e[k], string(filepath.ListSeparator))
}

type LockedWriteCloser struct {
	io.WriteCloser
	mu sync.Mutex
}

func (w *LockedWriteCloser) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.WriteCloser.Write(p)
}

func (w *LockedWriteCloser) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.WriteCloser.Close()
}

type LockedReadCloser struct {
	io.ReadCloser
	mu sync.Mutex
}

func (r *LockedReadCloser) Read(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.ReadCloser.Read(p)
}

func (r *LockedReadCloser) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.ReadCloser.Close()
}

type NopReadWriteCloser struct {
	io.Reader
	io.Writer
	io.Closer
}

func (n NopReadWriteCloser) Read(p []byte) (int, error) {
	if n.Reader != nil {
		return n.Reader.Read(p)
	}
	return 0, io.EOF
}

func (n NopReadWriteCloser) Write(p []byte) (int, error) {
	if n.Writer != nil {
		return n.Writer.Write(p)
	}
	return len(p), nil
}

func (n NopReadWriteCloser) Close() error {
	if n.Closer != nil {
		return n.Closer.Close()
	}
	return nil
}

func IsParentDir(parentDir, childPath string) bool {
	p, err := filepath.Rel(parentDir, childPath)
	return err == nil && p != "." && !strings.HasPrefix(p, ".."+string(filepath.Separator))
}
