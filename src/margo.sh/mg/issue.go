package mg

import (
	"bytes"
	"fmt"
	"margo.sh/htm"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var (
	commonPatterns = struct {
		sync.RWMutex
		m map[Lang][]*regexp.Regexp
	}{
		m: map[Lang][]*regexp.Regexp{
			"": {
				regexp.MustCompile(`^\s*(?P<path>.+?\.\w+):(?P<line>\d+:)(?P<column>\d+:?)?(?P<message>.+)$`),
				regexp.MustCompile(`^\s*(?P<path>.+?\.\w+)\((?P<line>\d+)(?:,(?P<column>\d+))?\):(?P<message>.+)$`),
			},
		},
	}
)

type commonPattern struct {
	Lang     Lang
	Patterns []*regexp.Regexp
}

func AddCommonPatterns(lang Lang, l ...*regexp.Regexp) {
	p := &commonPatterns
	p.Lock()
	defer p.Unlock()

	for _, k := range []Lang{"", lang} {
		p.m[k] = append(p.m[k], l...)
	}
}

func CommonPatterns(langs ...Lang) []*regexp.Regexp {
	p := &commonPatterns
	p.RLock()
	defer p.RUnlock()

	l := p.m[""]
	l = l[:len(l):len(l)]
	for _, lang := range langs {
		l = append(l, p.m[lang]...)
	}
	return l
}

type IssueTag string

const (
	Error   = IssueTag("error")
	Warning = IssueTag("warning")
	Notice  = IssueTag("notice")
)

type issueHash struct {
	loc string
	row int
	msg string
}

type Issue struct {
	Path    string
	Name    string
	Row     int
	Col     int
	End     int
	Tag     IssueTag
	Label   string
	Message string
}

func (isu *Issue) finalize() Issue {
	v := *isu
	if v.Tag == "" {
		v.Tag = Error
	}
	return v
}

func (isu *Issue) hash() issueHash {
	h := issueHash{
		loc: isu.Path,
		row: isu.Row,
		msg: isu.Message,
	}
	if h.loc == "" {
		h.loc = isu.Name
	}
	return h
}

func (isu *Issue) Equal(p Issue) bool {
	return isu.hash() == p.hash()
}

func (isu *Issue) SameFile(p Issue) bool {
	if isu.Path != "" {
		return isu.Path == p.Path
	}
	return isu.Name == p.Name
}

func (isu *Issue) InView(v *View) bool {
	if isu.Path != "" && isu.Path == v.Path {
		return true
	}
	if isu.Name != "" && isu.Name == v.Name {
		return true
	}
	return false
}

func (isu *Issue) Valid() bool {
	return (isu.Name != "" || isu.Path != "") && isu.Message != ""
}

type IssueSet []Issue

func (s IssueSet) Equal(issues IssueSet) bool {
	if len(s) != len(issues) {
		return false
	}
	for _, p := range s {
		if !issues.Has(p) {
			return false
		}
	}
	return true
}

func (s IssueSet) Add(l ...Issue) IssueSet {
	m := make(map[issueHash]*Issue, len(s)+len(l))
	for _, lst := range []IssueSet{s, IssueSet(l)} {
		for i, _ := range lst {
			isu := &lst[i]
			m[isu.hash()] = isu
		}
	}
	s = make(IssueSet, 0, len(m))
	for _, isu := range m {
		s = append(s, isu.finalize())
	}
	return s
}

func (s IssueSet) Remove(l ...Issue) IssueSet {
	res := make(IssueSet, 0, len(s)+len(l))
	q := IssueSet(l)
	for _, p := range s {
		if !q.Has(p) {
			res = append(res, p)
		}
	}
	return res
}

func (s IssueSet) Has(p Issue) bool {
	for _, q := range s {
		if p.Equal(q) {
			return true
		}
	}
	return false
}

func (is IssueSet) AllInView(v *View) IssueSet {
	issues := make(IssueSet, 0, len(is))
	for _, i := range is {
		if i.InView(v) {
			issues = append(issues, i)
		}
	}
	return issues
}

type StoreIssues struct {
	ActionType

	IssueKey
	Issues IssueSet
}

type IssueKey struct {
	Key  interface{}
	Name string
	Path string
	Dir  string
}

type issueKeySupport struct {
	ReducerType
	issues map[IssueKey]IssueSet
}

func (iks *issueKeySupport) RMount(mx *Ctx) {
	iks.issues = map[IssueKey]IssueSet{}
}

func (iks *issueKeySupport) Reduce(mx *Ctx) *State {
	switch act := mx.Action.(type) {
	case StoreIssues:
		if len(act.Issues) == 0 {
			delete(iks.issues, act.IssueKey)
		} else {
			iks.issues[act.IssueKey] = act.Issues
		}
	}

	issues := IssueSet{}
	norm := filepath.Clean
	name := norm(mx.View.Name)
	path := norm(mx.View.Path)
	dir := norm(mx.View.Dir())
	match := func(k IssueKey) bool {
		// no restrictions were set
		k.Key = nil
		if k == (IssueKey{}) {
			return true
		}

		if path != "" && path == k.Path {
			return true
		}
		if name != "" && name == k.Name {
			return true
		}
		// if the view doesn't exist on disk, the dir is unreliable
		if path != "" && dir != "" && dir == k.Dir {
			return true
		}
		return false
	}
	for k, v := range iks.issues {
		if match(k) {
			issues = append(issues, v...)
		}
	}

	return mx.State.AddIssues(issues...)
}

type issueStatusSupport struct{ ReducerType }

func (_ issueStatusSupport) Reduce(mx *Ctx) *State {
	if len(mx.Issues) == 0 {
		return mx.State
	}

	type Cfg struct {
		title  string
		inView int
		total  int
	}
	cfgs := map[IssueTag]*Cfg{
		Error:   {title: "Errors"},
		Warning: {title: "Warning"},
		Notice:  {title: "Notices"},
	}

	msg := ""
	els := []htm.Element{}
	for _, isu := range mx.Issues {
		cfg, ok := cfgs[isu.Tag]
		if !ok {
			continue
		}

		cfg.total++
		if !isu.InView(mx.View) {
			continue
		}
		cfg.inView++

		if isu.Message == "" || isu.Row != mx.View.Row {
			continue
		}

		s := ""
		if isu.Label == "" {
			s = isu.Message
		} else {
			s = isu.Label + ": " + isu.Message
		}
		els = append(els, htm.Text(s))
		if len(msg) <= 1 {
			msg = s
		}
	}

	status := make([]string, 0, len(cfgs)+1)
	for _, k := range []IssueTag{Error, Warning, Notice} {
		cfg := cfgs[k]
		if cfg.total == 0 {
			continue
		}
		status = append(status, fmt.Sprintf("%d/%d %s", cfg.inView, cfg.total, cfg.title))
	}
	st := mx.State.AddHUD(
		htm.Span(nil,
			htm.A(&htm.AAttrs{Action: DisplayIssues{}}, htm.Text("Issues")),
			htm.Textf(" ( %s )", strings.Join(status, ", ")),
		),
		els...,
	)
	if msg != "" {
		status = append(status, msg)
	}
	return st.AddStatus(status...)
}

type IssueOut struct {
	Patterns []*regexp.Regexp
	Base     Issue
	Dir      string
	Done     chan<- struct{}

	buf    []byte
	mu     sync.Mutex
	issues IssueSet
	isu    *Issue
	pfx    []byte
	closed bool
}

func (w *IssueOut) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return 0, os.ErrClosed
	}

	w.buf = append(w.buf, p...)
	w.scan(false)
	return len(p), nil
}

func (w *IssueOut) Close() error {
	defer w.Flush()

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return os.ErrClosed
	}

	w.closed = true
	if w.Done != nil {
		close(w.Done)
	}

	return nil
}

func (w *IssueOut) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.flush()
	return nil
}

func (w *IssueOut) Issues() IssueSet {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.scan(true)
	issues := make(IssueSet, len(w.issues))
	copy(issues, w.issues)
	return issues
}

func (w *IssueOut) scan(scanTail bool) {
	lines := bytes.Split(w.buf, []byte{'\n'})
	var tail []byte
	if !scanTail {
		n := len(lines) - 1
		tail, lines = lines[n], lines[:n]
	}

	for _, ln := range lines {
		w.scanLine(bytes.TrimRight(ln, "\r"))
	}

	w.buf = append(w.buf[:0], tail...)
}

func (w *IssueOut) scanLine(ln []byte) {
	pfx := ln[:len(ln)-len(bytes.TrimLeft(ln, " \t"))]
	ind := bytes.TrimPrefix(pfx, w.pfx)
	if n := len(ind); n > 0 && w.isu != nil {
		w.isu.Message += "\n" + string(ln[len(pfx)-n:])
		return
	}
	w.flush()

	w.pfx = pfx
	ln = ln[len(pfx):]
	w.isu = w.match(ln)
}

func (w *IssueOut) flush() {
	if w.isu == nil {
		return
	}
	isu := *w.isu
	w.isu = nil
	if isu.Valid() && !w.issues.Has(isu) {
		w.issues = append(w.issues, isu)
	}
}

func (w *IssueOut) match(s []byte) *Issue {
	for _, p := range w.Patterns {
		if isu := w.matchOne(p, s); isu != nil {
			return isu
		}
	}
	return nil
}

func (w *IssueOut) matchOne(p *regexp.Regexp, s []byte) *Issue {
	submatch := p.FindSubmatch(s)
	if submatch == nil {
		return nil
	}

	str := func(s []byte) string {
		return string(bytes.Trim(s, ": \t\r\n"))
	}
	num := func(s []byte) int {
		if n, _ := strconv.Atoi(str(s)); n > 0 {
			return n - 1
		}
		return 0
	}

	isu := w.Base
	for i, k := range p.SubexpNames() {
		v := submatch[i]
		switch k {
		case "path":
			isu.Path = str(v)
			if isu.Path != "" && w.Dir != "" && !filepath.IsAbs(isu.Path) {
				isu.Path = filepath.Join(w.Dir, isu.Path)
			}
		case "line":
			isu.Row = num(v)
		case "column":
			isu.Col = num(v)
		case "end":
			isu.End = num(v)
		case "label":
			lbl := str(v)
			if lbl != "" {
				isu.Label = lbl
			}
		case "error", "warning", "notice":
			isu.Tag = IssueTag(k)
			isu.Message = str(v)
		case "message":
			isu.Message = str(v)
		case "tag":
			tag := IssueTag(str(v))
			if tag == Warning || tag == Error || tag == Notice {
				isu.Tag = tag
			}
		}
	}
	return &isu
}
