package mg

import (
	"os"
	"path/filepath"
	"strings"
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

func (e EnvMap) Merge(p map[string]string) EnvMap {
	if len(p) == 0 {
		return e
	}

	m := make(EnvMap, len(e)+len(p))
	for k, v := range e {
		m[k] = v
	}
	for k, v := range p {
		m[k] = v
	}
	return m
}

func (e EnvMap) Environ() []string {
	el := os.Environ()
	l := make([]string, 0, len(e)+len(el))
	for _, s := range el {
		k := strings.SplitN(s, "=", 2)[0]
		if _, exists := e[k]; !exists {
			l = append(l, s)
		}
	}
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

func IsParentDir(parentDir, childPath string) bool {
	p, err := filepath.Rel(parentDir, childPath)
	return err == nil && p != "." && !strings.HasPrefix(p, ".."+string(filepath.Separator))
}
