package htm

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/ugorji/go/codec"
	"io"
	"margo.sh/mg/actions"
	"strconv"
	"strings"
)

var (
	ArticleAttrs   = &Attrs{Class: class{"article"}}
	HeadingAttrs   = &Attrs{Class: class{"heading"}}
	HighlightAttrs = &Attrs{Class: class{"highlight"}}
	colonText      = Text(": ")

	esc = strings.NewReplacer(
		`&`, "&amp;",
		`<`, "&lt;",
		`>`, "&gt;",
	)
)

func Article(heading IElement, content ...Element) Element {
	cl := append(make([]Element, 0, 3), Span(HeadingAttrs, heading), colonText)
	switch len(content) {
	case 0:
	case 1:
		cl = append(cl, content[0])
	default:
		ul := make([]Element, len(content))
		for i, c := range content {
			ul[i] = Li(nil, c)
		}
		cl = append(cl, Ul(nil, ul...))
	}
	return Div(ArticleAttrs, cl...)
}

func Text(s string) IElement                         { return iRawNode{rawNode: rawNode{s: []byte(esc.Replace(s))}} }
func Textf(format string, a ...interface{}) IElement { return Text(fmt.Sprintf(format, a...)) }
func HighlightText(s string) IElement                { return Span(HighlightAttrs, Text(s)) }

func P(a *Attrs, l ...IElement) BElement    { return bnode{node{t: "p", a: a, l: ils(l)}} }
func Div(a *Attrs, l ...Element) BElement   { return bnode{node{t: "div", a: a, l: els(l)}} }
func Span(a *Attrs, l ...IElement) IElement { return inode{node{t: "span", a: a, l: ils(l)}} }
func A(a *AAttrs, l ...IElement) IElement   { return inode{node{t: "a", a: a, l: ils(l)}} }

func Ul(a *Attrs, l ...Element) BElement { return bnode{node{t: "ul", a: a, l: els(l)}} }
func Ol(a *Attrs, l ...Element) BElement { return bnode{node{t: "ol", a: a, l: els(l)}} }
func Li(a *Attrs, l ...Element) BElement { return bnode{node{t: "li", a: a, l: els(l)}} }

func Em(a *Attrs, l ...IElement) IElement     { return inode{node{t: "em", a: a, l: ils(l)}} }
func EmText(s string) IElement                { return Em(nil, Text(s)) }
func Strong(a *Attrs, l ...IElement) IElement { return inode{node{t: "strong", a: a, l: ils(l)}} }
func StrongText(s string) IElement            { return Strong(nil, Text(s)) }

func H1(a *Attrs, l ...Element) BElement { return bnode{node{t: "H1", a: a, l: els(l)}} }
func H2(a *Attrs, l ...Element) BElement { return bnode{node{t: "H2", a: a, l: els(l)}} }
func H3(a *Attrs, l ...Element) BElement { return bnode{node{t: "H3", a: a, l: els(l)}} }
func H4(a *Attrs, l ...Element) BElement { return bnode{node{t: "H4", a: a, l: els(l)}} }
func H5(a *Attrs, l ...Element) BElement { return bnode{node{t: "H5", a: a, l: els(l)}} }
func H6(a *Attrs, l ...Element) BElement { return bnode{node{t: "H6", a: a, l: els(l)}} }

type class struct{ s string }

type Attrs struct {
	Class class
}

func (a *Attrs) attrs() (string, error) {
	if a == nil || a.Class.s == "" {
		return "", nil
	}
	return `class=` + strconv.Quote(a.Class.s), nil
}

type AAttrs struct {
	Action actions.ClientAction
	Class  string
}

func (a *AAttrs) attrs() (string, error) {
	if a == nil {
		return "", nil
	}
	buf := bytes.Buffer{}
	if a.Class != "" {
		buf.WriteString(` class=`)
		buf.WriteString(strconv.Quote(a.Class))
	}
	if a.Action != nil {
		js := []byte{}
		err := codec.NewEncoderBytes(&js, &codec.JsonHandle{}).Encode(a.Action.ClientAction())
		if err != nil {
			return "", err
		}
		s := make([]byte, base64.StdEncoding.EncodedLen(len(js)))
		base64.StdEncoding.Encode(s, js)
		buf.WriteString(` href="data:application/json;base64,`)
		buf.Write(s)
		buf.WriteByte('"')
	}
	return strings.TrimSpace(buf.String()), nil
}

type attrs interface {
	attrs() (string, error)
}

type elementType struct{}

func (et elementType) element() {}

type Element interface {
	element()
	FPrintHTML(w io.Writer) error
	FPrintText(w io.Writer) error
}

type IElement interface {
	Element
	iElement()
}

type BElement interface {
	Element
	bElement()
}

type inode struct{ node }

func (inode) iElement() {}

type bnode struct{ node }

func (bnode) bElement() {}

func (b bnode) FPrintText(w io.Writer) error {
	if err := b.node.FPrintText(w); err != nil {
		return err
	}
	_, err := w.Write([]byte{'\n'})
	return err
}

type node struct {
	elementType
	t    string
	void bool
	a    attrs
	l    nodeList
}

func (n node) FPrintHTML(w io.Writer) error {
	if n.t != "" {
		attrs := ""
		if n.a != nil {
			s, err := n.a.attrs()
			if err != nil {
				return err
			}
			attrs = s
		}
		if _, err := io.WriteString(w, `<`+n.t+` `+attrs+`>`); err != nil {
			return err
		}
	}
	for i, l := 0, n.l; i < l.len(); i++ {
		if err := l.item(i).FPrintHTML(w); err != nil {
			return err
		}
	}
	if n.t != "" && !n.void {
		s := `</` + n.t + `>`
		if _, err := io.WriteString(w, s); err != nil {
			return err
		}
	}
	return nil
}

func (n node) FPrintText(w io.Writer) error {
	for i, l := 0, n.l; i < l.len(); i++ {
		if err := l.item(i).FPrintText(w); err != nil {
			return err
		}
	}
	return nil
}

type iRawNode struct {
	rawNode
	inode
}

type bRawNode struct {
	rawNode
	inode
}

type rawNode struct {
	elementType
	s []byte
}

func (rn rawNode) FPrintHTML(w io.Writer) error {
	_, err := w.Write(rn.s)
	return err
}

func (rn rawNode) FPrintText(w io.Writer) error {
	return rn.FPrintHTML(w)
}

type nodeList interface {
	len() int
	item(int) Element
}

type els []Element

func (l els) len() int           { return len(l) }
func (l els) item(i int) Element { return l[i] }

type ils []IElement

func (l ils) len() int           { return len(l) }
func (l ils) item(i int) Element { return l[i] }
