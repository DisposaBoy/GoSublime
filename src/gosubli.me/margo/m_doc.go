package main

import (
	"path/filepath"
)

type Doc struct {
	Src  string `json:"src"`
	Pkg  string `json:"pkg"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	Fn   string `json:"fn"`
	Row  int    `json:"row"`
	Col  int    `json:"col"`
}

type mDoc struct {
	Fn        string
	Src       interface{}
	Env       map[string]string
	Offset    int
	TabIndent bool
	TabWidth  int
	FindDef   bool
	FindUse   bool
	FindInfo  bool
}

func (m *mDoc) Call() (interface{}, string) {
	// get the path from our current filename
	res := m.findCode([]string{filepath.Dir(m.Fn)})
	return res, ""
}

func init() {
	registry.Register("doc", func(_ *Broker) Caller {
		return &mDoc{
			Env:     map[string]string{},
			FindDef: true,
		}
	})

	registry.Register("usage", func(_ *Broker) Caller {
		return &mDoc{
			Env:     map[string]string{},
			FindUse: true,
		}
	})
}
