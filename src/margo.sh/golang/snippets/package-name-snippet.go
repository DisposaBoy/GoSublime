package snippets

import (
	"margo.sh/golang/cursor"
	"margo.sh/golang/goutil"
	"margo.sh/mg"
	"regexp"
)

var (
	pkgDirNamePat = regexp.MustCompile(`(\w+)\W*$`)
)

func PackageNameSnippet(cx *cursor.CurCtx) []mg.Completion {
	if cx.PkgName != goutil.NilPkgName || cx.Scope != cursor.PackageScope {
		return nil
	}

	var cl []mg.Completion
	seen := map[string]bool{}
	add := func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		cl = append(cl, mg.Completion{
			Query: `package ` + name,
			Src: `
				package ` + name + `

				$0
			`,
		})
	}

	dir := cx.View.Dir()
	pkg, _ := goutil.BuildContext(cx.Ctx).ImportDir(dir, 0)
	if pkg != nil && pkg.Name != "" {
		add(pkg.Name)
	} else {
		add(pkgDirNamePat.FindString(dir))
	}

	cl = append(cl, mg.Completion{
		Query: `package main`,
		Title: `main{}`,
		Src: `
			package main

			func main() {
				$0
			}
		`,
	})

	add("main")

	return cl
}
