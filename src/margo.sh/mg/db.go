package mg

import (
	"sync"
)

var (
	_ KVStore = (KVStores)(nil)
	_ KVStore = (*Store)(nil)
	_ KVStore = (*KVMap)(nil)
)

// KVStore represents a generic key value store.
//
// All operations are safe for concurrent access.
//
// The main implementation in this and related packages is Store.
type KVStore interface {
	// Put stores the value in the store with identifier key
	// NOTE: use Del instead of storing nil
	Put(key, value interface{})

	// Get returns the value stored using identifier key
	// NOTE: if the value doesn't exist, nil is returned
	Get(key interface{}) interface{}

	// Del removes the value identified by key from the store
	Del(key interface{})
}

// KVStores implements a KVStore that duplicates its operations on a list of k/v stores
type KVStores []KVStore

// Put calls .Put on each of k/v stores in the list
func (kvl KVStores) Put(k, v interface{}) {
	for _, kvs := range kvl {
		kvs.Put(k, v)
	}
}

// Get returns the first value identified by k found in the list of k/v stores
func (kvl KVStores) Get(k interface{}) interface{} {
	for _, kvs := range kvl {
		if v := kvs.Get(k); v != nil {
			return v
		}
	}
	return nil
}

// Del removed the value identified by k from all k/v stores in the list
func (kvl KVStores) Del(k interface{}) {
	for _, kvs := range kvl {
		kvs.Del(k)
	}
}

// KVMap implements a KVStore using a map.
// The zero-value is safe for use with all operations.
type KVMap struct {
	vals map[interface{}]interface{}
	mu   sync.RWMutex
}

// Put implements KVStore.Put
func (m *KVMap) Put(k interface{}, v interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.vals == nil {
		m.vals = map[interface{}]interface{}{}
	}

	m.vals[k] = v
}

// Get implements KVStore.Get
func (m *KVMap) Get(k interface{}) interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.vals[k]
}

// Del implements KVStore.Del
func (m *KVMap) Del(k interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.vals, k)
}

// Clear removes all values from the store
func (m *KVMap) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.vals = map[interface{}]interface{}{}
}

// Values returns a copy of all values stored
func (m *KVMap) Values() map[interface{}]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vals := make(map[interface{}]interface{}, len(m.vals))
	for k, v := range m.vals {
		vals[k] = v
	}
	return vals
}
