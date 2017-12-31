package margo_pkg

import (
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"gosublime/something-borrowed/types"
	"regexp"
	"strconv"
)

type mLintReport struct {
	Fn      string
	Row     int
	Col     int
	Message string
	Kind    string
}

type mLint struct {
	Dir jString
	Fn  jString
	Src jString
	v   struct {
		dir string
		fn  string
		src string
	}
	Filter []string

	fset    *token.FileSet
	af      *ast.File
	reports []mLintReport
}

var (
	mLintErrPat = regexp.MustCompile(`(.+?):(\d+):(\d+): (.+)`)
	mLinters    = map[string]func(kind string, m *mLint){
		"gs.flag.parse": mLintCheckFlagParse,
		"gs.types":      mLintCheckTypes,
	}
)

func (m *mLint) Call() (interface{}, string) {
	m.v.fn = m.Fn.String()
	m.v.dir = m.Dir.String()
	m.v.src = m.Src.String()

	filterKind := map[string]bool{}
	for _, kind := range m.Filter {
		filterKind[kind] = true
	}

	var err error
	m.reports = []mLintReport{}
	m.fset, m.af, err = parseAstFile(m.v.fn, m.v.src, parser.DeclarationErrors)
	if err == nil {
		for kind, f := range mLinters {
			if !filterKind[kind] {
				f(kind, m)
			}
		}
	} else if el, ok := err.(scanner.ErrorList); ok && !filterKind["gs.syntax"] {
		for _, e := range el {
			m.report(mLintReport{
				Fn:      m.v.fn,
				Row:     e.Pos.Line - 1,
				Col:     e.Pos.Column - 1,
				Message: e.Msg,
				Kind:    "gs.syntax",
			})
		}
	}

	res := M{
		"reports": m.reports,
	}
	return res, ""
}

func (m *mLint) report(reps ...mLintReport) {
	m.reports = append(m.reports, reps...)
}

func init() {
	registry.Register("lint", func(_ *Broker) Caller {
		return &mLint{}
	})
}

func mLintCheckFlagParse(kind string, m *mLint) {
	reps := []mLintReport{}
	foundParse := false
	ast.Inspect(m.af, func(node ast.Node) bool {
		switch c := node.(type) {
		case *ast.CallExpr:
			if sel, ok := c.Fun.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "flag" {
					switch sel.Sel.String() {
					case "Parse":
						foundParse = true
					case "Var", "Bool", "BoolVar", "String", "StringVar", "Int", "IntVar", "Uint", "UintVar", "Int64", "Int64Var", "Uint64", "Uint64Var", "Duration", "DurationVar", "Float64", "Float64Var":

						if !foundParse && c != nil {
							tp := m.fset.Position(c.Pos())
							if tp.IsValid() {
								reps = append(reps, mLintReport{
									Fn:      tp.Filename,
									Row:     tp.Line - 1,
									Col:     tp.Column - 1,
									Message: "Cannot find corresponding call to flag.Parse()",
									Kind:    kind,
								})
							}
						}
					}
				}
			}
		}
		return !foundParse
	})

	if !foundParse {
		m.report(reps...)
	}
}

func mLintCheckTypes(kind string, m *mLint) {
	files := []*ast.File{m.af}
	if m.v.dir != "" {
		pkg, pkgs, _ := parsePkg(m.fset, m.v.dir, parser.ParseComments)
		if pkg == nil {
			for _, p := range pkgs {
				if f := p.Files[m.v.fn]; f != nil {
					pkg = p
					break
				}
			}

			if pkg == nil {
				return
			}
		}

		for fn, f := range pkg.Files {
			if fn != m.v.fn {
				files = append(files, f)
			}
		}
	}

	ctx := types.Context{
		Error: func(err error) {
			s := mLintErrPat.FindStringSubmatch(err.Error())
			if len(s) == 5 {
				line, _ := strconv.Atoi(s[2])
				column, _ := strconv.Atoi(s[3])

				m.report(mLintReport{
					Fn:      s[1],
					Row:     line - 1,
					Col:     column - 1,
					Message: s[4],
					Kind:    kind,
				})
			}
		},
	}

	ctx.Check(m.fset, files)
}
