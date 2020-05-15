package web

import (
	"margo.sh/format"
	"margo.sh/mg"
	"strings"
)

var (
	// PrettierDefaultLangs is the list of languages used if Prettier.Langs is empty.
	PrettierDefaultLangs = []mg.Lang{
		mg.CSS,
		mg.HTML,
		mg.JS,
		mg.JSON,
		mg.JSX,
		mg.SVG,
		mg.TS,
		mg.TSX,
		mg.XML,
	}
)

// Prettier is a reducer that does code fmt'ing using https://github.com/prettier/prettier
// By default it fmt's CSS, HTML, JS, JSON, JSX, SVG, TS, TSX and XML files.
//
// NOTE: as a special-case, files with extensions starting with `.sublime-` are ignored.
// NOTE: you will need to install prettier separately
type Prettier struct {
	mg.ReducerType

	// Langs is the list of languages to fmt.
	// It's empty (len==0), PrettierDefaultLangs is used.
	Langs []mg.Lang
}

func (p *Prettier) Reduce(mx *mg.Ctx) *mg.State {
	if strings.HasPrefix(mx.View.Ext, ".sublime-") {
		return mx.State
	}
	langs := p.Langs
	if len(langs) == 0 {
		langs = PrettierDefaultLangs
	}
	return format.FmtCmd{
		Langs:   langs,
		Actions: []mg.Action{mg.ViewFmt{}, mg.ViewPreSave{}},
		Name:    "prettier",
		Args:    []string{"--stdin-filepath", mx.View.Filename()},
	}.Reduce(mx)
}
