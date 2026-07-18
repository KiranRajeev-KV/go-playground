package kvstore

import (
	"errors"
	"maps"
	"sync"
)

// Store is a concurrency-safe key-value store.
// The zero value for Store is ready to use.
// Store synchronizes access to its internal map only. It does not synchronize
// access to mutable data referenced by stored values and does not deep-copy
// values. Callers are responsible for synchronizing access to such data.
// A Store must not be copied after first use.
type Store[K comparable, V any] struct {
	store map[K]V
	mu    sync.RWMutex
}

var (
	// ErrValueNotFound is returned when a requested key is not present.
	ErrValueNotFound = errors.New("value not found")

	// ErrNilUpdateFunc is returned when Update is called with a nil callback.
	ErrNilUpdateFunc = errors.New("nil update callback")
)

// New returns an initialized, empty Store.
func New[K comparable, V any]() *Store[K, V] {
	return &Store[K, V]{
		store: make(map[K]V),
	}
}

// Get returns the value associated with key.
// Get returns the stored value by assignment. It does not deep-copy the value
// or any data referenced by it.
// It returns ErrValueNotFound if key is not present.
func (s *Store[K, V]) Get(key K) (V, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, ok := s.store[key]
	if !ok {
		var zero V
		return zero, ErrValueNotFound
	}
	return val, nil
}

// Set associates value with key, replacing any existing value.
// Set stores value by assignment. It does not deep-copy value or any data
// referenced by value.
func (s *Store[K, V]) Set(key K, value V) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.store == nil {
		s.store = make(map[K]V)
	}

	s.store[key] = value
}

// Delete removes the value associated with key.
// It returns ErrValueNotFound if key is not present.
func (s *Store[K, V]) Delete(key K) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.store[key]
	if !ok {
		return ErrValueNotFound
	}

	delete(s.store, key)
	return nil
}

// LoadOrStore returns the existing value for key if present.
// Otherwise, it stores and returns the supplied value.
// LoadOrStore stores and returns values by assignment. It does not deep-copy
// values or any data referenced by them.
// The loaded result reports whether an existing value was returned.
func (s *Store[K, V]) LoadOrStore(key K, value V) (V, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	val, ok := s.store[key]
	if ok {
		return val, true
	}

	if s.store == nil {
		s.store = make(map[K]V)
	}

	s.store[key] = value
	return value, false
}

// Update replaces the value associated with key with the result of fn.
// Update returns ErrNilUpdateFunc if fn is nil, regardless of whether key is
// present. It returns ErrValueNotFound if key is not present and fn is non-nil.
// Update calls fn while holding the Store's exclusive lock. The callback must
// not call methods on s or wait for any goroutine that may call methods on s.
func (s *Store[K, V]) Update(key K, fn func(V) V) error {
	if fn == nil {
		return ErrNilUpdateFunc
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	val, ok := s.store[key]
	if !ok {
		return ErrValueNotFound
	}

	s.store[key] = fn(val)
	return nil
}

// Snapshot returns a shallow copy of the Store's current entries.
// The map itself is independent, but keys, values, and referenced data are
// copied using ordinary assignment and may remain shared.
func (s *Store[K, V]) Snapshot() map[K]V {
	s.mu.RLock()
	defer s.mu.RUnlock()

	snapshot := make(map[K]V, len(s.store))
	maps.Copy(snapshot, s.store)
	return snapshot
}

// Len returns the number of entries present when it is called.
// The returned count is a point-in-time observation and can become stale after
// Len returns.
func (s *Store[K, V]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.store == nil {
		return 0
	}

	return len(s.store)
}
