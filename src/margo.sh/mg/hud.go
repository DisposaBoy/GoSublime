package mg

import (
	"bytes"
	"margo.sh/htm"
)

type HUDState struct {
	Articles []string
}

func (h HUDState) AddArticle(heading htm.IElement, content ...htm.Element) HUDState {
	buf := &bytes.Buffer{}
	htm.Article(heading, content...).FPrintHTML(buf)
	l := h.Articles
	h.Articles = append(l[:len(l):len(l)], buf.String())
	return h
}
