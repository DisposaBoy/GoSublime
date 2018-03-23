package mg

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
)

var (
	CommonPatterns = []*regexp.Regexp{
		regexp.MustCompile(`^\s*(?P<path>.+?\.\w+):(?P<line>\d+:)(?P<column>\d+:?)?(?P<message>.+)$`),
		regexp.MustCompile(`^\s*(?P<path>.+?\.\w+)\((?P<line>\d+)(?:,(?P<column>\d+))?\):(?P<message>.+)$`),
	}
)

type IssueTag string

const (
	IssueError   = IssueTag("error")
	IssueWarning = IssueTag("warning")
)

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

func (isu Issue) Equal(p Issue) bool {
	return isu.SameFile(p) && isu.Row == p.Row && isu.Message == p.Message
}

func (isu Issue) SameFile(p Issue) bool {
	if isu.Path != "" {
		return isu.Path == p.Path
	}
	return isu.Name == p.Name
}

func (isu Issue) InView(v *View) bool {
	if isu.Path != "" {
		return v.Path == isu.Path
	}
	return isu.Name == v.Name
}

func (isu Issue) Valid() bool {
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
	res := make(IssueSet, 0, len(s)+len(l))
	for _, lst := range []IssueSet{s, IssueSet(l)} {
		for _, p := range lst {
			if !res.Has(p) {
				res = append(res, p)
			}
		}
	}
	return res
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

type issueSupport struct{}

func (_ issueSupport) Reduce(mx *Ctx) *State {
	if len(mx.Issues) == 0 {
		return mx.State
	}

	status := make([]string, 0, 3)
	status = append(status, "placeholder")
	inview := 0
	for _, isu := range mx.Issues {
		if !isu.InView(mx.View) {
			continue
		}
		inview++
		if len(status) > 1 || isu.Message == "" || isu.Row != mx.View.Row {
			continue
		}
		if isu.Label != "" {
			status = append(status, isu.Label)
		}
		status = append(status, isu.Message)
	}
	status[0] = fmt.Sprintf("Issues (%d/%d)", inview, len(mx.Issues))
	return mx.AddStatus(status...)
}

type IssueWriter struct {
	Writer   io.Writer
	Patterns []*regexp.Regexp
	Base     Issue
	Dir      string

	buf    []byte
	mu     sync.Mutex
	issues IssueSet
	isu    *Issue
	pfx    []byte
}

func (w *IssueWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.buf = append(w.buf, p...)
	w.scan(false)

	if w.Writer != nil {
		return w.Writer.Write(p)
	}
	return len(p), nil
}

func (w *IssueWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.flush()
	return nil
}

func (w *IssueWriter) Issues() IssueSet {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.scan(true)
	issues := make(IssueSet, len(w.issues))
	copy(issues, w.issues)
	return issues
}

func (w *IssueWriter) scan(scanTail bool) {
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

func (w *IssueWriter) scanLine(ln []byte) {
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

func (w *IssueWriter) flush() {
	if w.isu == nil {
		return
	}
	isu := *w.isu
	w.isu = nil
	if isu.Valid() && !w.issues.Has(isu) {
		w.issues = append(w.issues, isu)
	}
}

func (w *IssueWriter) match(s []byte) *Issue {
	for _, p := range w.Patterns {
		if isu := w.matchOne(p, s); isu != nil {
			return isu
		}
	}
	return nil
}

func (w *IssueWriter) matchOne(p *regexp.Regexp, s []byte) *Issue {
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
			isu.Label = str(v)
		case "error", "warning":
			isu.Tag = IssueTag(k)
			isu.Message = str(v)
		case "message":
			isu.Message = str(v)
		case "tag":
			tag := IssueTag(str(v))
			if tag == IssueWarning || tag == IssueError {
				isu.Tag = tag
			}
		}
	}
	return &isu
}
