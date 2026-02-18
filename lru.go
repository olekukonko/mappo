// lru.go
package mappo

import (
	"container/list"
	"sync"
	"time"
)

// LRUConfig holds configuration for LRU map.
type LRUConfig[K comparable, V any] struct {
	MaxSize    int
	TTL        time.Duration
	OnEviction func(key K, value V)
	Concurrent bool
}

// LRU provides a fixed-size LRU map with optional TTL.
type LRU[K comparable, V any] struct {
	mu         sync.RWMutex
	items      map[K]*list.Element
	order      *list.List
	maxSize    int
	defaultTTL time.Duration
	onEviction func(K, V)
	muEnabled  bool
}

type lruElement[K comparable, V any] struct {
	key        K
	value      V
	expiration time.Time
}

// NewLRU creates a new LRU map.
func NewLRU[K comparable, V any](maxSize int) *LRU[K, V] {
	return NewLRUWithConfig[K, V](LRUConfig[K, V]{
		MaxSize:    maxSize,
		Concurrent: true,
	})
}

// NewLRUWithConfig creates a new LRU map with configuration.
func NewLRUWithConfig[K comparable, V any](cfg LRUConfig[K, V]) *LRU[K, V] {
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = 1000
	}

	return &LRU[K, V]{
		items:      make(map[K]*list.Element),
		order:      list.New(),
		maxSize:    cfg.MaxSize,
		defaultTTL: cfg.TTL,
		onEviction: cfg.OnEviction,
		muEnabled:  cfg.Concurrent,
	}
}

// Set adds or updates a value with default TTL.
func (l *LRU[K, V]) Set(key K, value V) {
	l.SetWithTTL(key, value, l.defaultTTL)
}

// SetWithTTL adds or updates a value with specific TTL.
func (l *LRU[K, V]) SetWithTTL(key K, value V, ttl time.Duration) {
	if l.muEnabled {
		l.mu.Lock()
		defer l.mu.Unlock()
	}

	var expiration time.Time
	if ttl > 0 {
		expiration = time.Now().Add(ttl)
	}

	// Check if key already exists
	if elem, exists := l.items[key]; exists {
		// Move to front (most recent)
		l.order.MoveToFront(elem)
		// Update value
		elem.Value.(*lruElement[K, V]).value = value
		elem.Value.(*lruElement[K, V]).expiration = expiration
		return
	}

	// Add new element
	elem := l.order.PushFront(&lruElement[K, V]{
		key:        key,
		value:      value,
		expiration: expiration,
	})
	l.items[key] = elem

	// Evict if over capacity
	if l.order.Len() > l.maxSize {
		l.evictBack()
	}
}

// Get retrieves a value.
func (l *LRU[K, V]) Get(key K) (V, bool) {
	if l.muEnabled {
		l.mu.Lock()
		defer l.mu.Unlock()
	}

	elem, exists := l.items[key]
	if !exists {
		var zero V
		return zero, false
	}

	item := elem.Value.(*lruElement[K, V])

	// Check expiration
	if !item.expiration.IsZero() && time.Now().After(item.expiration) {
		l.removeElement(elem)
		var zero V
		return zero, false
	}

	// Move to front (most recent)
	l.order.MoveToFront(elem)
	return item.value, true
}

// Peek returns a value without updating its LRU status.
func (l *LRU[K, V]) Peek(key K) (V, bool) {
	if l.muEnabled {
		l.mu.RLock()
		defer l.mu.RUnlock()
	}

	elem, exists := l.items[key]
	if !exists {
		var zero V
		return zero, false
	}

	item := elem.Value.(*lruElement[K, V])

	// Check expiration
	if !item.expiration.IsZero() && time.Now().After(item.expiration) {
		// Need write lock for removal
		if l.muEnabled {
			l.mu.RUnlock()
			l.mu.Lock()
			defer l.mu.Unlock()
			if elem, exists := l.items[key]; exists {
				if !elem.Value.(*lruElement[K, V]).expiration.IsZero() &&
					time.Now().After(elem.Value.(*lruElement[K, V]).expiration) {
					l.removeElement(elem)
				}
			}
		} else {
			l.removeElement(elem)
		}
		var zero V
		return zero, false
	}

	return item.value, true
}

// Delete removes a key.
func (l *LRU[K, V]) Delete(key K) bool {
	if l.muEnabled {
		l.mu.Lock()
		defer l.mu.Unlock()
	}

	elem, exists := l.items[key]
	if !exists {
		return false
	}

	l.removeElement(elem)
	return true
}

// Has returns true if the key exists and is not expired.
func (l *LRU[K, V]) Has(key K) bool {
	if l.muEnabled {
		l.mu.RLock()
		defer l.mu.RUnlock()
	}

	elem, exists := l.items[key]
	if !exists {
		return false
	}

	item := elem.Value.(*lruElement[K, V])
	if !item.expiration.IsZero() && time.Now().After(item.expiration) {
		return false
	}
	return true
}

// Len returns the number of items.
func (l *LRU[K, V]) Len() int {
	if l.muEnabled {
		l.mu.RLock()
		defer l.mu.RUnlock()
	}
	return l.order.Len()
}

// Clear removes all items.
func (l *LRU[K, V]) Clear() {
	if l.muEnabled {
		l.mu.Lock()
		defer l.mu.Unlock()
	}

	if l.onEviction != nil {
		for _, elem := range l.items {
			item := elem.Value.(*lruElement[K, V])
			l.onEviction(item.key, item.value)
		}
	}

	l.items = make(map[K]*list.Element)
	l.order.Init()
}

// evictBack removes the least recently used item.
func (l *LRU[K, V]) evictBack() {
	elem := l.order.Back()
	if elem != nil {
		l.removeElement(elem)
	}
}

// removeElement removes an element and calls onEviction if set.
func (l *LRU[K, V]) removeElement(elem *list.Element) {
	item := elem.Value.(*lruElement[K, V])
	delete(l.items, item.key)
	l.order.Remove(elem)

	if l.onEviction != nil {
		l.onEviction(item.key, item.value)
	}
}

// PurgeExpired removes all expired items.
func (l *LRU[K, V]) PurgeExpired() int {
	if l.muEnabled {
		l.mu.Lock()
		defer l.mu.Unlock()
	}

	now := time.Now()
	removed := 0

	for _, elem := range l.items {
		item := elem.Value.(*lruElement[K, V])
		if !item.expiration.IsZero() && now.After(item.expiration) {
			l.removeElement(elem)
			removed++
		}
	}
	return removed
}

// Keys returns all keys in order from most to least recent.
func (l *LRU[K, V]) Keys() []K {
	if l.muEnabled {
		l.mu.RLock()
		defer l.mu.RUnlock()
	}

	keys := make([]K, 0, l.order.Len())
	for e := l.order.Front(); e != nil; e = e.Next() {
		keys = append(keys, e.Value.(*lruElement[K, V]).key)
	}
	return keys
}

// ForEach iterates over items from most to least recent.
func (l *LRU[K, V]) ForEach(fn func(K, V) bool) {
	if l.muEnabled {
		l.mu.RLock()
		defer l.mu.RUnlock()
	}

	for e := l.order.Front(); e != nil; e = e.Next() {
		item := e.Value.(*lruElement[K, V])
		if !fn(item.key, item.value) {
			return
		}
	}
}
