package actions

import (
	"reflect"
	"sync"
)

// ActionCreator creates a new action.
type ActionCreator func(ActionData) (Action, error)

// Registry is a map of known action creators for actions coming from the client
type Registry struct {
	mu sync.RWMutex
	m  map[string]ActionCreator
}

// Lookup returns the action creator named name or nil if doesn't exist.
func (r *Registry) Lookup(name string) ActionCreator {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.m[name]
}

// Register is equivalent of RegisterCreator(name, MakeActionCreator(zero)).
func (r *Registry) Register(name string, zero Action) *Registry {
	return r.RegisterCreator(name, MakeActionCreator(zero))
}

// RegisterCreator registers the action creator f.
//
// NOTE: If a function is already registered with that name, it panics.
func (r *Registry) RegisterCreator(name string, f ActionCreator) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.m[name]; exists {
		panic("ActionCreator " + name + " is already registered")
	}

	if r.m == nil {
		r.m = map[string]ActionCreator{}
	}

	r.m[name] = f
	return r
}

// MakeActionCreator returns an action creator that decodes the ActionData into a copy of zero.
func MakeActionCreator(zero Action) ActionCreator {
	z := reflect.ValueOf(zero)
	t := z.Type()
	return func(d ActionData) (_ Action, err error) {
		p := reflect.New(t)
		v := p.Elem()
		v.Set(z)
		if len(d.Data) != 0 {
			err = d.Decode(p.Interface())
		}
		return v.Interface().(Action), err
	}
}
