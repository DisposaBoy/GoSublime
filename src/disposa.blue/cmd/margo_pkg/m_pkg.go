package margo_pkg

import (
	"go/parser"
)

type mPkg struct {
	Fn  string
	Src string
}

func (m *mPkg) Call() (interface{}, string) {
	res := M{}
	_, af, err := parseAstFile(m.Fn, m.Src, parser.PackageClauseOnly)
	if err == nil {
		res["name"] = af.Name.String()
	}
	return res, errStr(err)
}

func init() {
	registry.Register("pkg", func(_ *Broker) Caller {
		return &mPkg{}
	})
}
