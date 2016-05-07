package main

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	buildIgnore = regexp.MustCompile(`^\W*[+]build.*?\bignore\b`)
)

func ignoreNm(name string) bool {
	if name == "" || name[0] == '.' || name[0] == '_' {
		return true
	}

	name = strings.ToLower(name)
	return strings.HasSuffix(name, "_test.go")
}

func ls(fn string) ([]string, bool) {
	nm := filepath.Base(fn)
	if ignoreNm(nm) {
		return nil, false
	}

	d, err := os.Open(fn)
	if err != nil {
		return nil, false
	}
	defer d.Close()

	names, err := d.Readdirnames(-1)

	return names, (err == nil || len(names) > 0)
}

func walk(root string, ch chan string, dir string) {
	names, ok := ls(dir)
	if !ok {
		return
	}

	for _, nm := range names {
		if ignoreNm(nm) {
			continue
		}

		fn := filepath.Join(dir, nm)
		isFx, isGo := fx(nm)
		if isGo {
			ch <- fn
		} else if !isFx {
			walk(root, ch, fn)
		}
	}
}

func pkgPaths(srcDir string, exclude []string) map[string]string {
	paths := map[string]string{}
	done := make(chan struct{})
	ch := make(chan string, 100)
	fset := token.NewFileSet()
	seen := map[string]void{}
	excluded := map[string]void{}

	for _, s := range exclude {
		excluded[s] = void{}
	}

	proc := func(fn string) {
		dir := filepath.Dir(fn)
		p, err := filepath.Rel(srcDir, dir)
		if err != nil || strings.HasPrefix(p, ".") {
			return
		}
		p = filepath.ToSlash(p)

		if _, ok := paths[p]; ok {
			return
		}

		if _, ok := seen[p]; ok {
			return
		}

		af, _ := parser.ParseFile(fset, fn, nil, parser.ImportsOnly|parser.ParseComments)
		if af == nil || af.Name == nil {
			return
		}

		name := af.Name.String()

		for _, cg := range af.Comments {
			for _, c := range cg.List {
				if buildIgnore.MatchString(c.Text) {
					return
				}
			}
		}

		if _, skip := excluded[name]; skip {
			seen[p] = void{}
			return
		}

		paths[p] = name
	}

	go func() {
		defer close(done)
		for fn := range ch {
			proc(fn)
		}
	}()

	walk(srcDir, ch, srcDir)
	close(ch)
	<-done

	return paths
}
