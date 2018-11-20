package mg

type HUDArticle struct {
	Title   string
	Content []string
}

type HUDState struct {
	Articles []HUDArticle
}

func (h HUDState) Add(l ...HUDArticle) HUDState {
	h.Articles = append(h.Articles[:len(h.Articles):len(h.Articles)], l...)
	return h
}
