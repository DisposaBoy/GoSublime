package vfs

import (
	"sort"
)

type NodeList struct{ l []*Node }

func (n *NodeList) Sorted() *NodeList {
	x := n.Copy()
	sort.Slice(x.l, func(i, j int) bool { return x.l[i].name < x.l[j].name })
	return x
}

func (n *NodeList) Filter(f func(nd *Node) bool) *NodeList {
	if n == nil {
		return &NodeList{}
	}
	var l []*Node
	for i, nd := range n.l {
		if !f(nd) {
			continue
		}
		if len(l) == 0 {
			l = make([]*Node, 0, len(n.l)-i)
		}
		l = append(l, nd)
	}
	return &NodeList{l: l}
}

func (n *NodeList) Len() int {
	if n == nil {
		return 0
	}
	return len(n.l)
}

func (n *NodeList) Copy() *NodeList {
	x := &NodeList{}
	if n != nil {
		x.l = append(([]*Node)(nil), n.l...)
	}
	return x
}

func (n *NodeList) Nodes() []*Node {
	if n == nil {
		return nil
	}
	return n.l
}

func (n *NodeList) Node(name string) *Node {
	if n == nil {
		return nil
	}
	_, c := n.Find(name)
	return c
}

func (n *NodeList) Find(name string) (index int, c *Node) {
	if n == nil {
		return -1, nil
	}
	for i, c := range n.l {
		if c.name == name {
			return i, c
		}
	}
	return -1, nil
}

func (n *NodeList) Index(nd *Node) (index int) {
	if n == nil {
		return -1
	}
	for i, c := range n.l {
		if c == nd {
			return i
		}
	}
	return -1
}

func (n *NodeList) Add(nd *Node) *NodeList {
	if n == nil {
		return &NodeList{l: []*Node{nd}}
	}
	x := &NodeList{l: make([]*Node, len(n.l)+1)}
	x.l[0] = nd
	copy(x.l[1:], n.l)
	return x
}

func (n *NodeList) Some(f func(nd *Node) bool) bool {
	if n == nil {
		return false
	}
	for _, c := range n.l {
		if f(c) {
			return true
		}
	}
	return false
}
