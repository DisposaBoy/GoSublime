package mg

import (
	"sync"
)

type Reducer func(State, Action) State

type Listener func(State)

type Store struct {
	mu        sync.Mutex
	state     State
	listeners []*struct{ Listener }
	reducers  []Reducer
}

func (s *Store) Dispatch(a Action) {
	go s.reduce(a)
}

func (s *Store) reduce(a Action) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := s.state
	state.EphemeralState = EphemeralState{}
	for _, r := range s.reducers {
		state = r(state, a)
	}
	s.state = state

	for _, p := range s.listeners {
		p.Listener(state)
	}
}

func (s *Store) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.state
}

func NewStore() *Store {
	return &Store{}
}

func (s *Store) Subscribe(l Listener) (unsubscribe func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p := &struct{ Listener }{l}
	s.listeners = append(s.listeners[:len(s.listeners):len(s.listeners)], p)

	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		listeners := make([]*struct{ Listener }, 0, len(s.listeners)-1)
		for _, q := range s.listeners {
			if p != q {
				listeners = append(listeners, q)
			}
		}
		s.listeners = listeners
	}
}

func (s *Store) Reducers(reducers ...Reducer) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(reducers) != 0 {
		s.reducers = reducers
	}
}
