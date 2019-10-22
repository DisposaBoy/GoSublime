package mgutil

import (
	"sync"
)

type memo struct {
	sync.Mutex
	k, v interface{}
}

func (m *memo) value() interface{} {
	m.Lock()
	defer m.Unlock()

	return m.v
}

func (m *memo) read(new func() interface{}) interface{} {
	m.Lock()
	defer m.Unlock()

	if m.v != nil {
		return m.v
	}
	m.v = new()
	return m.v
}

type Memo struct {
	mu sync.Mutex
	ml []*memo
}

func (m *Memo) index(k interface{}) (int, *memo) {
	for i, p := range m.ml {
		if p.k == k {
			return i, p
		}
	}
	return -1, nil
}

func (m *Memo) memo(k interface{}) *memo {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, p := m.index(k)
	if p == nil {
		p = &memo{k: k}
		m.ml = append(m.ml, p)
	}
	return p
}

func (m *Memo) Read(k interface{}, new func() interface{}) interface{} {
	if m == nil {
		return new()
	}
	return m.memo(k).read(new)
}

func (m *Memo) Del(k interface{}) {
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

func (m *Memo) Clear() {
	if m == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.ml = nil
}

func (m *Memo) Values() map[interface{}]interface{} {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	vals := make(map[interface{}]interface{}, len(m.ml))
	for k, p := range m.ml {
		if v := p.value(); v != nil {
			vals[k] = v
		}
	}
	return vals
}
