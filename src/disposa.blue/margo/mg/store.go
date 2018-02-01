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
	listener  Listener
	reducers  []Reducer
	cfg       func() EditorConfig
}

func (s *Store) Dispatch(act Action) {
	go s.dispatch(act, true)
}

func (s *Store) dispatch(act Action, callListener bool) State {
	state := s.prepState()

	s.mu.Lock()
	reducers := s.reducers
	s.mu.Unlock()

	for _, r := range reducers {
		state = r(state, act)
	}

	s.mu.Lock()
	listener := s.listener
	listeners := s.listeners
	s.state = state
	s.mu.Unlock()

	if callListener && listener != nil {
		listener(state)
	}

	for _, p := range listeners {
		p.Listener(state)
	}

	return state
}

func (s *Store) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.state
}

func (s *Store) prepState() State {
	s.mu.Lock()
	defer s.mu.Unlock()

	st := s.state
	st.EphemeralState = EphemeralState{}
	if s.cfg != nil {
		st.Config = s.cfg()
	}
	return st
}

func newStore(l Listener) *Store {
	return &Store{listener: l}
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

func (s *Store) Use(reducers ...Reducer) *Store {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.reducers = append(s.reducers[:len(s.reducers):len(s.reducers)], reducers...)
	return s
}

func (s *Store) EditorConfig(f func() EditorConfig) *Store {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cfg = f
	return s
}
