package mappo

import (
	"container/list"
	"sync"

	"github.com/puzpuzpuz/xsync/v3"
)

// Ordered provides a map that maintains insertion order.
// It's safe for concurrent use when created with Concurrent option.
type Ordered[K comparable, V any] struct {
	mu        sync.RWMutex
	items     *xsync.MapOf[K, *orderedElement[K, V]]
	order     *list.List
	muEnabled bool

	// Per-instance pool for orderedElement (no global state)
	// Note: list.Element cannot be pooled due to unexported fields
	elemPool *sync.Pool
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
	o := &Ordered[K, V]{
		items:     xsync.NewMapOf[K, *orderedElement[K, V]](),
		order:     list.New(),
		muEnabled: cfg.Concurrent,
	}

	// Initialize per-instance pool for orderedElement
	if cfg.Concurrent {
		o.elemPool = &sync.Pool{
			New: func() any {
				return &orderedElement[K, V]{}
			},
		}
	}

	return o
}

// getOrderedElement gets an orderedElement from pool or allocates new.
func (o *Ordered[K, V]) getOrderedElement() *orderedElement[K, V] {
	if o.elemPool != nil {
		if e := o.elemPool.Get(); e != nil {
			elem := e.(*orderedElement[K, V])
			elem.element = nil // Clear reference
			return elem
		}
	}
	return &orderedElement[K, V]{}
}

// putOrderedElement returns orderedElement to pool.
func (o *Ordered[K, V]) putOrderedElement(e *orderedElement[K, V]) {
	if o.elemPool != nil && e != nil {
		e.element = nil // Clear reference to allow GC
		o.elemPool.Put(e)
	}
}

// Set adds or updates a key-value pair, maintaining insertion order.
// If the key already exists, its position is not changed.
func (o *Ordered[K, V]) Set(key K, value V) {
	if o.muEnabled {
		o.mu.Lock()
		defer o.mu.Unlock()
	}

	if elem, exists := o.items.Load(key); exists {
		elem.Value = value
		return
	}

	// New key - get from pool or allocate
	oe := o.getOrderedElement()
	oe.Key = key
	oe.Value = value

	// list.Element must be allocated fresh (unexported fields)
	e := o.order.PushBack(oe)
	oe.element = e

	o.items.Store(key, oe)
}

// SetFront adds or updates a key-value pair at the front of the order.
func (o *Ordered[K, V]) SetFront(key K, value V) {
	if o.muEnabled {
		o.mu.Lock()
		defer o.mu.Unlock()
	}

	if elem, exists := o.items.Load(key); exists {
		// Move to front
		o.order.MoveToFront(elem.element)
		elem.Value = value
		return
	}

	// New key - add to front
	oe := o.getOrderedElement()
	oe.Key = key
	oe.Value = value

	e := o.order.PushFront(oe)
	oe.element = e

	o.items.Store(key, oe)
}

// SetBack adds or updates a key-value pair at the back of the order.
func (o *Ordered[K, V]) SetBack(key K, value V) {
	o.Set(key, value) // Already adds to back
}

// Get retrieves a value by key.
func (o *Ordered[K, V]) Get(key K) (V, bool) {
	elem, exists := o.items.Load(key)
	if !exists {
		var zero V
		return zero, false
	}
	return elem.Value, true
}

// GetAt returns the key-value pair at the given index (0-based).
// O(n) operation - use sparingly.
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

// IndexOf returns the index of a key, or -1 if not found.
// O(n) operation.
func (o *Ordered[K, V]) IndexOf(key K) int {
	if o.muEnabled {
		o.mu.RLock()
		defer o.mu.RUnlock()
	}

	idx := 0
	for e := o.order.Front(); e != nil; e = e.Next() {
		elem := e.Value.(*orderedElement[K, V])
		if elem.Key == key {
			return idx
		}
		idx++
	}
	return -1
}

// Delete removes a key.
func (o *Ordered[K, V]) Delete(key K) bool {
	if o.muEnabled {
		o.mu.Lock()
		defer o.mu.Unlock()
	}

	elem, exists := o.items.Load(key)
	if !exists {
		return false
	}

	o.order.Remove(elem.element)
	o.items.Delete(key)
	o.putOrderedElement(elem)
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
	o.items.Delete(elem.Key)
	o.putOrderedElement(elem)
	return true
}

// MoveToFront moves an existing key to the front.
func (o *Ordered[K, V]) MoveToFront(key K) bool {
	if o.muEnabled {
		o.mu.Lock()
		defer o.mu.Unlock()
	}

	elem, exists := o.items.Load(key)
	if !exists {
		return false
	}

	o.order.MoveToFront(elem.element)
	return true
}

// MoveToBack moves an existing key to the back.
func (o *Ordered[K, V]) MoveToBack(key K) bool {
	if o.muEnabled {
		o.mu.Lock()
		defer o.mu.Unlock()
	}

	elem, exists := o.items.Load(key)
	if !exists {
		return false
	}

	o.order.MoveToBack(elem.element)
	return true
}

// InsertBefore inserts key before the mark key.
func (o *Ordered[K, V]) InsertBefore(key, mark K, value V) bool {
	if o.muEnabled {
		o.mu.Lock()
		defer o.mu.Unlock()
	}

	markElem, exists := o.items.Load(mark)
	if !exists {
		return false
	}

	// Remove old if exists
	if oldElem, exists := o.items.Load(key); exists {
		o.order.Remove(oldElem.element)
		o.putOrderedElement(oldElem)
	}

	oe := o.getOrderedElement()
	oe.Key = key
	oe.Value = value

	e := o.order.InsertBefore(oe, markElem.element)
	oe.element = e

	o.items.Store(key, oe)
	return true
}

// InsertAfter inserts key after the mark key.
func (o *Ordered[K, V]) InsertAfter(key, mark K, value V) bool {
	if o.muEnabled {
		o.mu.Lock()
		defer o.mu.Unlock()
	}

	markElem, exists := o.items.Load(mark)
	if !exists {
		return false
	}

	// Remove old if exists
	if oldElem, exists := o.items.Load(key); exists {
		o.order.Remove(oldElem.element)
		o.putOrderedElement(oldElem)
	}

	oe := o.getOrderedElement()
	oe.Key = key
	oe.Value = value

	e := o.order.InsertAfter(oe, markElem.element)
	oe.element = e

	o.items.Store(key, oe)
	return true
}

// Swap swaps two elements by index.
func (o *Ordered[K, V]) Swap(i, j int) bool {
	if o.muEnabled {
		o.mu.Lock()
		defer o.mu.Unlock()
	}

	if i < 0 || j < 0 || i >= o.order.Len() || j >= o.order.Len() {
		return false
	}

	// Get elements at indices
	var elemI, elemJ *list.Element
	idx := 0
	for e := o.order.Front(); e != nil; e = e.Next() {
		if idx == i {
			elemI = e
		}
		if idx == j {
			elemJ = e
		}
		idx++
	}

	if elemI == nil || elemJ == nil {
		return false
	}

	// Swap values in the orderedElements
	oi := elemI.Value.(*orderedElement[K, V])
	oj := elemJ.Value.(*orderedElement[K, V])

	oi.Key, oj.Key = oj.Key, oi.Key
	oi.Value, oj.Value = oj.Value, oi.Value

	// Update map pointers
	o.items.Store(oi.Key, oi)
	o.items.Store(oj.Key, oj)

	return true
}

// Reverse reverses the order in place.
func (o *Ordered[K, V]) Reverse() {
	if o.muEnabled {
		o.mu.Lock()
		defer o.mu.Unlock()
	}

	// Build slice of elements
	elems := make([]*orderedElement[K, V], 0, o.order.Len())
	for e := o.order.Front(); e != nil; e = e.Next() {
		elems = append(elems, e.Value.(*orderedElement[K, V]))
	}

	// Clear and re-add in reverse
	o.order.Init()
	for i := len(elems) - 1; i >= 0; i-- {
		elem := elems[i]

		e := o.order.PushBack(elem)
		elem.element = e
		o.items.Store(elem.Key, elem)
	}
}

// Has returns true if the key exists.
func (o *Ordered[K, V]) Has(key K) bool {
	_, exists := o.items.Load(key)
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

	// Return elements to pool
	if o.elemPool != nil {
		for e := o.order.Front(); e != nil; e = e.Next() {
			elem := e.Value.(*orderedElement[K, V])
			o.putOrderedElement(elem)
		}
	}

	o.items.Clear()
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

// PopFront removes and returns the first element.
func (o *Ordered[K, V]) PopFront() (K, V, bool) {
	if o.muEnabled {
		o.mu.Lock()
		defer o.mu.Unlock()
	}

	if o.order.Len() == 0 {
		var zeroK K
		var zeroV V
		return zeroK, zeroV, false
	}

	e := o.order.Front()
	elem := e.Value.(*orderedElement[K, V])
	o.order.Remove(e)
	o.items.Delete(elem.Key)
	o.putOrderedElement(elem)
	return elem.Key, elem.Value, true
}

// PopBack removes and returns the last element.
func (o *Ordered[K, V]) PopBack() (K, V, bool) {
	if o.muEnabled {
		o.mu.Lock()
		defer o.mu.Unlock()
	}

	if o.order.Len() == 0 {
		var zeroK K
		var zeroV V
		return zeroK, zeroV, false
	}

	e := o.order.Back()
	elem := e.Value.(*orderedElement[K, V])
	o.order.Remove(e)
	o.items.Delete(elem.Key)
	o.putOrderedElement(elem)
	return elem.Key, elem.Value, true
}
