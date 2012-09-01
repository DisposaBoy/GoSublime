package main

import (
	"go/parser"
)

type AcPackageNameArgs struct {
	Fn  string `json:"fn"`
	Src string `json:"src"`
}

type AcPackageResult struct {
	Name string
	Path string
}

func init() {
	act(Action{
		Path: "package",
		Doc:  "",
		Func: func(r Request) (data, error) {
			a := AcPackageNameArgs{}
			if err := r.Decode(&a); err != nil {
				return "", err
			}
			res := AcPackageResult{}
			_, af, err := parseAstFile(a.Fn, a.Src, parser.PackageClauseOnly)
			if err == nil {
				res.Name = af.Name.String()
				// res.Path = af.
			}
			return res, err
		},
	})
}
