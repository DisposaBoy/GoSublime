package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
)

func usageSimple(t *testing.T, offset int, output []*Doc) {
	// get current dir
	wd, _ := os.Getwd()
	fn := filepath.Join(wd, "testing", "simple.go")
	dec := &mDoc{
		Fn:        fn,
		Src:       nil,
		Env:       map[string]string{},
		Offset:    offset,
		TabIndent: true,
		TabWidth:  4,
		FindUse:   true,
	}

	raw, _ := dec.Call()
	assertOut(t, raw, output)
}
func findSimple(t *testing.T, offset int, output []*Doc) {
	// get current dir
	wd, _ := os.Getwd()
	fn := filepath.Join(wd, "testing", "simple.go")
	dec := &mDoc{
		Fn:        fn,
		Src:       nil,
		Env:       map[string]string{},
		Offset:    offset,
		TabIndent: true,
		TabWidth:  4,
		FindDef:   true,
	}

	raw, _ := dec.Call()
	assertOut(t, raw, output)
}

func assertOut(t *testing.T, raw interface{}, output []*Doc) {
	if assert.Len(t, raw, len(output)) {
		docs := raw.([]*Doc)

		for i := 0; i < len(docs); i++ {
			assert.Equal(t, output[i].Col, docs[i].Col, "Col")
			assert.Equal(t, output[i].Kind, docs[i].Kind, "Kind")
			assert.Equal(t, output[i].Name, docs[i].Name, "Name")
			assert.Equal(t, output[i].Pkg, docs[i].Pkg, "Pkg")
			assert.Equal(t, output[i].Row, docs[i].Row, "Row")
			assert.Equal(t, output[i].Src, docs[i].Src, "Src")
			assert.Regexp(t, output[i].Fn+"$", docs[i].Fn)
		}
	} else {
		pretty.Logf("Found: %# v", raw)
	}
}

func TestCallBasics_Struct(t *testing.T) {
	findSimple(
		t,
		85,
		//[]*Doc{&Doc{Src: "a struct {\n\tvalue string\n}", Pkg: "tstm", Name: "a", Kind: "type", Fn: "testing.go", Row: 4, Col: 5}},
		[]*Doc{&Doc{Src: "", Pkg: "testing", Name: "a", Kind: "struct", Fn: "simple.go", Row: 4, Col: 5}},
	)
}

func TestCallBasics_StructFieldSet(t *testing.T) {
	findSimple(
		t,
		95,
		//[]*Doc{&Doc{Src: "value string", Pkg: "tstm", Name: "value", Kind: "field", Fn: "testing.go", Row: 5, Col: 1}},
		[]*Doc{&Doc{Src: "", Pkg: "testing", Name: "value", Kind: "field", Fn: "simple.go", Row: 5, Col: 1}},
	)
}

func TestCallBasics_Framework(t *testing.T) {
	findSimple(
		t,
		115,
		//[]*Doc{&Doc{Src: "func Println()", Pkg: "fmt", Name: "Println", Kind: "func", Fn: "/usr/local/go/src/pkg/fmt/print.go", Row: 262, Col: 5}},
		[]*Doc{&Doc{Src: "", Pkg: "fmt", Name: "Println", Kind: "func", Fn: "print.go", Row: 262, Col: 5}},
	)
}

func TestUsages_StructField(t *testing.T) {
	usageSimple(
		t,
		95,
		[]*Doc{
			&Doc{Src: "", Pkg: "testing", Name: "value", Kind: "field", Fn: "simple.go", Row: 5, Col: 1},
			&Doc{Src: "", Pkg: "testing", Name: "value", Kind: "field", Fn: "simple.go", Row: 10, Col: 3},
			&Doc{Src: "", Pkg: "testing", Name: "value", Kind: "field", Fn: "simple.go", Row: 11, Col: 15},
		},
	)
}

func TestUsages_Framework(t *testing.T) {
	usageSimple(
		t,
		115,
		[]*Doc{&Doc{Src: "", Pkg: "fmt", Name: "Println", Kind: "func", Fn: "simple.go", Row: 11, Col: 5}},
	)
}

func TestUsages_Import(t *testing.T) {
	usageSimple(
		t,
		26,
		[]*Doc{&Doc{Src: "", Pkg: "testing", Name: "fmt", Kind: "package", Fn: "simple.go", Row: 11, Col: 1}},
	)
}
