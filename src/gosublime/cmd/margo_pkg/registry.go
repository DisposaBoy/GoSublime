package margo_pkg

import (
	"sync"
)

var (
	registry = &Registry{m: map[string]Method{}}
)

type Method func(*Broker) Caller

type Caller interface {
	Call() (res interface{}, err string)
}

type Registry struct {
	m   map[string]Method
	lck sync.RWMutex
}

func (r *Registry) Register(name string, method Method) {
	r.lck.Lock()
	defer r.lck.Unlock()

	if name == "" {
		panic("Cannot register method without a name")
	}
	if method == nil {
		panic("Method " + name + " is nil")
	}
	if r.m[name] != nil {
		panic("Method " + name + " is already registered")
	}

	r.m[name] = method
}

func (r *Registry) Lookup(name string) Method {
	r.lck.RLock()
	defer r.lck.RUnlock()
	return r.m[name]
}
