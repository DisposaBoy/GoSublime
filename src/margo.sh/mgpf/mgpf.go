package mgpf

import (
	"fmt"
	"io"
	"margo.sh/mgutil"
	"sync"
	"time"
)

var (
	enabled = &mgutil.AtomicBool{}

	DefaultPrintOpts = PrintOpts{
		Indent: "\t",
	}
)

func Enabled() bool {
	return enabled.IsSet()
}

func Enable() {
	enabled.Set(true)
}

func Disable() {
	enabled.Set(false)
}

type Dur struct {
	time.Duration
}

func (d Dur) String() string {
	p := d.Duration
	switch {
	case p < time.Millisecond:
		return p.Round(time.Microsecond).String()
	case p < time.Minute:
		return p.Round(time.Millisecond).String()
	default:
		return p.Round(time.Second).String()
	}
}

func D(d time.Duration) Dur {
	return Dur{Duration: d}
}

type PrintOpts struct {
	Prefix      string
	Indent      string
	MinDuration time.Duration
}

type Node struct {
	Name     string
	Duration time.Duration
	Samples  int
	Children []*Node

	start time.Time
}

func (n *Node) Dur() Dur {
	return D(n.duration())
}

func (n *Node) child(name string) *Node {
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	c := &Node{Name: name}
	n.Children = append(n.Children, c)
	return c
}

func (n *Node) duration() time.Duration {
	d := n.Duration
	if d > 0 {
		return d
	}

	for _, c := range n.Children {
		d += c.Duration
	}
	return d
}

func (n *Node) fprint(w io.Writer, o PrintOpts) {
	subTitle := ""
	if i := n.Samples; i >= 2 {
		subTitle = fmt.Sprintf("*%d", i)
	}
	fmt.Fprintf(w, "%s%s%s %s\n", o.Prefix, n.Name, subTitle, n.Dur())

	o.Prefix += o.Indent
	for _, c := range n.Children {
		if c.Duration >= o.MinDuration {
			c.fprint(w, o)
		}
	}
}

type Profile struct {
	root  *Node
	stack []*Node
	mu    sync.RWMutex
}

func (p *Profile) Dur() Dur {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.root.Dur()
}

func (p *Profile) Do(name string, f func()) {
	defer p.Push(name).Pop()
	f()
}

func (p *Profile) Push(name string) *Profile {
	p.update(func() {
		n := p.stack[len(p.stack)-1].child(name)
		n.start = time.Now()
		p.stack = append(p.stack, n)
	})
	return p
}

func (p *Profile) Pop() {
	p.update(func() {
		n := p.stack[len(p.stack)-1]
		n.Duration += time.Since(n.start)
		n.Samples++
		p.stack = p.stack[:len(p.stack)-1]
	})
}

func (p *Profile) Sample(name string, d time.Duration) {
	p.update(func() {
		n := p.stack[len(p.stack)-1].child(name)
		n.Duration += d
		n.Samples++
	})
}

func (p *Profile) Fprint(w io.Writer, opts *PrintOpts) {
	p.mu.Lock()
	defer p.mu.Unlock()

	o := DefaultPrintOpts
	if opts != nil {
		o = *opts
		if o.Indent == "" {
			o.Indent = DefaultPrintOpts.Indent
		}
	}

	p.root.fprint(w, o)
}

func (p *Profile) SetName(name string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.root.Name = name
}

func (p *Profile) update(f func()) {
	if !Enabled() {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	f()
}

func NewProfile(name string) *Profile {
	n := &Node{Name: name}
	return &Profile{root: n, stack: []*Node{n}}
}

func Since(t time.Time) Dur {
	return D(time.Since(t))
}
