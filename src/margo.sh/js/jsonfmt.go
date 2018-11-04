package js

import (
	"encoding/json"
	"margo.sh/mg"
)

type JsonFmt struct {
	mg.ReducerType

	Prefix string
	Indent string
}

func (j JsonFmt) ReCond(mx *mg.Ctx) bool {
	return mx.ActionIs(mg.ViewFmt{}) && mx.LangIs(mg.JSON)
}

func (j JsonFmt) Reduce(mx *mg.Ctx) *mg.State {
	fn := mx.View.Filename()
	r, err := mx.View.Open()
	if err != nil {
		return mx.AddErrorf("failed to open %s: %s\n", fn, err)
	}
	defer r.Close()

	var v interface{}
	if err := json.NewDecoder(r).Decode(&v); err != nil {
		return mx.AddErrorf("failed to unmarshal json %s: %s\n", fn, err)
	}

	src, err := json.MarshalIndent(v, j.Prefix, j.Indent)
	if err != nil {
		return mx.AddErrorf("failed to marshal json %s: %s\n", fn, err)
	}
	return mx.SetViewSrc(src)
}
