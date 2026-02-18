// ordered.go
package mappo

import (
	"container/list"
	"sync"
)

// Ordered provides a map that maintains insertion order.
// It's safe for concurrent use when created with WithMutex option.
type Ordered[K comparable, V any] struct {
	mu        sync.RWMutex
	items     map[K]*orderedElement[K, V]
	order     *list.List
	muEnabled bool
}

type orderedElement[K comparable, V any] struct {
	Key     K
	Value   V
	element *list.Element
}

// OrderedConfig holds configuration for Ordered map.
type OrderedConfig struct {
	// Concurrent enables mutex protection for concurrent use
	Concurrent bool
}

// NewOrdered creates a new ordered map.
func NewOrdered[K comparable, V any]() *Ordered[K, V] {
	return NewOrderedWithConfig[K, V](OrderedConfig{})
}

// NewOrderedWithConfig creates a new ordered map with configuration.
func NewOrderedWithConfig[K comparable, V any](cfg OrderedConfig) *Ordered[K, V] {
	return &Ordered[K, V]{
		items:     make(map[K]*orderedElement[K, V]),
		order:     list.New(),
		muEnabled: cfg.Concurrent,
	}
}

// Set adds or updates a key-value pair, maintaining insertion order.
// If the key already exists, its position is not changed.
func (o *Ordered[K, V]) Set(key K, value V) {
	if o.muEnabled {
		o.mu.Lock()
		defer o.mu.Unlock()
	}

	if elem, exists := o.items[key]; exists {
		elem.Value = value
		return
	}

	// New key - add to order
	e := &orderedElement[K, V]{
		Key:   key,
		Value: value,
	}
	e.element = o.order.PushBack(e)
	o.items[key] = e
}

// SetFront adds or updates a key-value pair at the front of the order.
func (o *Ordered[K, V]) SetFront(key K, value V) {
	if o.muEnabled {
		o.mu.Lock()
		defer o.mu.Unlock()
	}

	if elem, exists := o.items[key]; exists {
		// Move to front
		o.order.MoveToFront(elem.element)
		elem.Value = value
		return
	}

	// New key - add to front
	e := &orderedElement[K, V]{
		Key:   key,
		Value: value,
	}
	e.element = o.order.PushFront(e)
	o.items[key] = e
}

// SetBack adds or updates a key-value pair at the back of the order.
func (o *Ordered[K, V]) SetBack(key K, value V) {
	o.Set(key, value) // Already adds to back
}

// Get retrieves a value by key.
func (o *Ordered[K, V]) Get(key K) (V, bool) {
	if o.muEnabled {
		o.mu.RLock()
		defer o.mu.RUnlock()
	}

	elem, exists := o.items[key]
	if !exists {
		var zero V
		return zero, false
	}
	return elem.Value, true
}

// GetAt returns the key-value pair at the given index (0-based).
func (o *Ordered[K, V]) GetAt(index int) (K, V, bool) {
	if o.muEnabled {
		o.mu.RLock()
		defer o.mu.RUnlock()
	}

	if index < 0 || index >= o.order.Len() {
		var zeroK K
		var zeroV V
		return zeroK, zeroV, false
	}

	e := o.order.Front()
	for i := 0; i < index; i++ {
		e = e.Next()
	}
	elem := e.Value.(*orderedElement[K, V])
	return elem.Key, elem.Value, true
}

// Delete removes a key.
func (o *Ordered[K, V]) Delete(key K) bool {
	if o.muEnabled {
		o.mu.Lock()
		defer o.mu.Unlock()
	}

	elem, exists := o.items[key]
	if !exists {
		return false
	}

	o.order.Remove(elem.element)
	delete(o.items, key)
	return true
}

// DeleteAt removes the element at the given index.
func (o *Ordered[K, V]) DeleteAt(index int) bool {
	if o.muEnabled {
		o.mu.Lock()
		defer o.mu.Unlock()
	}

	if index < 0 || index >= o.order.Len() {
		return false
	}

	e := o.order.Front()
	for i := 0; i < index; i++ {
		e = e.Next()
	}
	elem := e.Value.(*orderedElement[K, V])
	o.order.Remove(e)
	delete(o.items, elem.Key)
	return true
}

// Has returns true if the key exists.
func (o *Ordered[K, V]) Has(key K) bool {
	if o.muEnabled {
		o.mu.RLock()
		defer o.mu.RUnlock()
	}

	_, exists := o.items[key]
	return exists
}

// Len returns the number of items.
func (o *Ordered[K, V]) Len() int {
	if o.muEnabled {
		o.mu.RLock()
		defer o.mu.RUnlock()
	}
	return o.order.Len()
}

// Clear removes all items.
func (o *Ordered[K, V]) Clear() {
	if o.muEnabled {
		o.mu.Lock()
		defer o.mu.Unlock()
	}

	o.items = make(map[K]*orderedElement[K, V])
	o.order.Init()
}

// Keys returns all keys in order.
func (o *Ordered[K, V]) Keys() []K {
	if o.muEnabled {
		o.mu.RLock()
		defer o.mu.RUnlock()
	}

	keys := make([]K, 0, o.order.Len())
	for e := o.order.Front(); e != nil; e = e.Next() {
		keys = append(keys, e.Value.(*orderedElement[K, V]).Key)
	}
	return keys
}

// Values returns all values in order.
func (o *Ordered[K, V]) Values() []V {
	if o.muEnabled {
		o.mu.RLock()
		defer o.mu.RUnlock()
	}

	values := make([]V, 0, o.order.Len())
	for e := o.order.Front(); e != nil; e = e.Next() {
		values = append(values, e.Value.(*orderedElement[K, V]).Value)
	}
	return values
}

// ForEach iterates over items in order. Return false to stop.
func (o *Ordered[K, V]) ForEach(fn func(K, V) bool) {
	if o.muEnabled {
		o.mu.RLock()
		defer o.mu.RUnlock()
	}

	for e := o.order.Front(); e != nil; e = e.Next() {
		elem := e.Value.(*orderedElement[K, V])
		if !fn(elem.Key, elem.Value) {
			return
		}
	}
}

// Front returns the first key-value pair.
func (o *Ordered[K, V]) Front() (K, V, bool) {
	if o.muEnabled {
		o.mu.RLock()
		defer o.mu.RUnlock()
	}

	if o.order.Len() == 0 {
		var zeroK K
		var zeroV V
		return zeroK, zeroV, false
	}

	elem := o.order.Front().Value.(*orderedElement[K, V])
	return elem.Key, elem.Value, true
}

// Back returns the last key-value pair.
func (o *Ordered[K, V]) Back() (K, V, bool) {
	if o.muEnabled {
		o.mu.RLock()
		defer o.mu.RUnlock()
	}

	if o.order.Len() == 0 {
		var zeroK K
		var zeroV V
		return zeroK, zeroV, false
	}

	elem := o.order.Back().Value.(*orderedElement[K, V])
	return elem.Key, elem.Value, true
}
