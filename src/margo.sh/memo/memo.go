package memo

import (
	"sync"
	"time"
)

type K = interface{}

type V = interface{}

type Sticky interface {
	InvalidateMemo(invalidatedAt time.Time)
}

type memo struct {
	k K
	sync.Mutex
	v V
}

func (m *memo) value() V {
	m.Lock()
	defer m.Unlock()

	return m.v
}

type M struct {
	mu sync.Mutex
	ml []*memo
}

func (m *M) index(k K) (int, *memo) {
	for i, p := range m.ml {
		if p.k == k {
			return i, p
		}
	}
	return -1, nil
}

func (m *M) memo(k K) *memo {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, p := m.index(k)
	if p == nil {
		p = &memo{k: k}
		m.ml = append(m.ml, p)
	}
	return p
}

func (m *M) Read(k K, new func() V) V {
	if m == nil {
		return new()
	}

	p := m.memo(k)
	p.Lock()
	defer p.Unlock()

	if p.v != nil {
		return p.v
	}
	p.v = new()
	if p.v != nil {
		return p.v
	}
	m.Del(k)
	return nil
}

func (m *M) Del(k K) {
	if m == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	i, _ := m.index(k)
	if i < 0 {
		return
	}

	m.ml[i] = m.ml[len(m.ml)-1]
	m.ml[len(m.ml)-1] = nil
	m.ml = m.ml[:len(m.ml)-1]
}

func (m *M) Clear() {
	if m == nil {
		return
	}

	invAt := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	ml := m.ml
	m.ml = nil
	stkl := []Sticky{}
	for _, p := range ml {
		if stk, ok := p.value().(Sticky); ok {
			m.ml = append(m.ml, p)
			stkl = append(stkl, stk)
		}
	}
	if len(stkl) == 0 {
		return
	}
	go func() {
		for _, stk := range stkl {
			stk.InvalidateMemo(invAt)
		}
	}()
}

func (m *M) Values() map[K]V {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	vals := make(map[K]V, len(m.ml))
	for k, p := range m.ml {
		if v := p.value(); v != nil {
			vals[k] = v
		}
	}
	return vals
}
