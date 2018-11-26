package mgutil

import (
	"os"
	"path/filepath"
	"strings"
)

// EnvMap is a map of environment variables
type EnvMap map[string]string

// copy returns a copy of the map
// sizeHint is a hint about the expected new size of the map
// if sizeHint is less than 0, it's assumed to be 0
func (e EnvMap) copy(sizeHint int) EnvMap {
	n := len(e) + sizeHint
	if n < 0 {
		n = 0
	}
	m := make(EnvMap, n)
	for k, v := range e {
		m[k] = v
	}
	return m
}

// Add is an alias of Set
func (e EnvMap) Add(k, v string) EnvMap {
	return e.Set(k, v)
}

// Set sets the key k in the map to the value v and a the new map
func (e EnvMap) Set(k, v string) EnvMap {
	m := e.copy(1)
	m[k] = v
	return m
}

// Unset removes the list of keys from the map and returns the new map
func (e EnvMap) Unset(keys ...string) EnvMap {
	m := e.copy(0)
	for _, k := range keys {
		delete(m, k)
	}
	return m
}

// Merge merges p into the map and returns a the new map
func (e EnvMap) Merge(p map[string]string) EnvMap {
	if len(p) == 0 {
		return e
	}

	m := e.copy(len(p))
	for k, v := range p {
		m[k] = v
	}
	return m
}

// Environ returns a copy of os.Environ merged with the values in the map
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

// Get returns the value for k if it exists in the map.
// If it doesn't exists or is an empty string, def is returned.
func (e EnvMap) Get(k, def string) string {
	if v := e[k]; v != "" {
		return v
	}
	return def
}

// Getenv returns the value for k if it exists in the map or via os.Getenv.
// If it doesn't exists or is an empty string, def is returned.
func (e EnvMap) Getenv(k, def string) string {
	if v := e[k]; v != "" {
		return v
	}
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// List is an alias of EventMap.PathList
func (e EnvMap) List(k string) []string {
	return e.PathList(k)
}

// PathList is the equivalent of PathList(e[k])
func (e EnvMap) PathList(k string) []string {
	return PathList(e[k])
}

// PathList splits s by filepath.ListSeparator and returns the list with empty and duplicate components removed
func PathList(s string) []string {
	l := strings.Split(s, string(filepath.ListSeparator))
	j := 0
	for i, p := range l {
		if p != "" && !StrSet(l[:i]).Has(p) {
			l[j] = p
			j++
		}
	}
	return l[:j:j]
}

// IsParentDir returns true if parentDir is a parent of childPath
func IsParentDir(parentDir, childPath string) bool {
	p, err := filepath.Rel(parentDir, childPath)
	return err == nil && p != "." && !strings.HasPrefix(p, ".."+string(filepath.Separator))
}
