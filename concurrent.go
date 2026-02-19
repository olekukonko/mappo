package mappo

import (
	"time"

	"github.com/puzpuzpuz/xsync/v3"
)

// Concurrent provides a high-performance concurrent map with optional TTL support.
// It wraps xsync.MapOf for optimal performance in high-concurrency scenarios.
type Concurrent[K comparable, V any] struct {
	m *xsync.MapOf[K, *concurrentEntry[V]]
}

type concurrentEntry[V any] struct {
	value      V
	expiration int64 // UnixNano, 0 means no expiration
}

// NewConcurrent creates a new concurrent map.
func NewConcurrent[K comparable, V any]() *Concurrent[K, V] {
	return &Concurrent[K, V]{
		m: xsync.NewMapOf[K, *concurrentEntry[V]](),
	}
}

// Get retrieves a value. Returns false if key doesn't exist or is expired.
func (c *Concurrent[K, V]) Get(key K) (V, bool) {
	entry, ok := c.m.Load(key)
	if !ok || entry == nil {
		var zero V
		return zero, false
	}

	// Check expiration
	if entry.expiration > 0 && nowNano() > entry.expiration {
		c.m.Delete(key)
		var zero V
		return zero, false
	}

	return entry.value, true
}

// Set stores a value with no expiration.
func (c *Concurrent[K, V]) Set(key K, value V) {
	c.m.Store(key, &concurrentEntry[V]{value: value})
}

// SetTTL stores a value with TTL.
func (c *Concurrent[K, V]) SetTTL(key K, value V, ttl time.Duration) {
	var exp int64
	if ttl > 0 {
		exp = time.Now().Add(ttl).UnixNano()
	}
	c.m.Store(key, &concurrentEntry[V]{value: value, expiration: exp})
}

// SetIfAbsent sets the value only if the key doesn't exist.
// Returns the actual value and true if loaded (already existed).
func (c *Concurrent[K, V]) SetIfAbsent(key K, value V) (V, bool) {
	var zero V
	entry := &concurrentEntry[V]{value: value}

	actual, loaded := c.m.LoadOrStore(key, entry)
	if loaded {
		return actual.value, true
	}
	return zero, false
}

// Compute allows atomic read-modify-write operations.
func (c *Concurrent[K, V]) Compute(key K, fn func(current V, exists bool) (newValue V, keep bool)) V {
	c.m.Compute(key, func(oldEntry *concurrentEntry[V], exists bool) (*concurrentEntry[V], bool) {
		var oldV V
		existsAndValid := exists && oldEntry != nil

		if existsAndValid {
			if oldEntry.expiration > 0 && nowNano() > oldEntry.expiration {
				existsAndValid = false
			} else {
				oldV = oldEntry.value
			}
		}

		newV, keep := fn(oldV, existsAndValid)
		if !keep {
			return nil, true // delete=true: remove the entry
		}

		return &concurrentEntry[V]{value: newV}, false // delete=false: store the entry
	})

	// After Compute, read back the actual stored value
	// This handles CAS retries correctly
	val, _ := c.Get(key)
	return val
}

// Delete removes a key.
func (c *Concurrent[K, V]) Delete(key K) bool {
	_, existed := c.m.Load(key)
	if existed {
		c.m.Delete(key)
	}
	return existed
}

// Has returns true if the key exists and is not expired.
func (c *Concurrent[K, V]) Has(key K) bool {
	_, ok := c.Get(key)
	return ok
}

// Len returns an estimate of the number of items.
func (c *Concurrent[K, V]) Len() int {
	return c.m.Size()
}

// Clear removes all items.
func (c *Concurrent[K, V]) Clear() {
	c.m.Clear()
}

// Range iterates over all items. Return false to stop.
// Expired items are skipped and deleted.
func (c *Concurrent[K, V]) Range(fn func(K, V) bool) {
	now := nowNano()
	c.m.Range(func(key K, entry *concurrentEntry[V]) bool {
		if entry.expiration > 0 && now > entry.expiration {
			c.m.Delete(key)
			return true
		}
		return fn(key, entry.value)
	})
}

// Keys returns all non-expired keys.
func (c *Concurrent[K, V]) Keys() []K {
	keys := make([]K, 0, c.Len())
	c.Range(func(k K, _ V) bool {
		keys = append(keys, k)
		return true
	})
	return keys
}

// Values returns all non-expired values.
func (c *Concurrent[K, V]) Values() []V {
	values := make([]V, 0, c.Len())
	c.Range(func(_ K, v V) bool {
		values = append(values, v)
		return true
	})
	return values
}

// Update performs an atomic read-modify-write and returns the new value.
// Semantically equivalent to Compute(fn) but signals "always keep" intent.
// API matches Sharded.Update
func (c *Concurrent[K, V]) Update(key K, fn func(current V, exists bool) V) V {
	return c.Compute(key, func(curr V, exists bool) (V, bool) {
		return fn(curr, exists), true
	})
}

// GetOrSet returns the existing value for the key if present, otherwise sets and returns the given value.
// API matches Sharded.GetOrSet
func (c *Concurrent[K, V]) GetOrSet(key K, val V) (actual V, loaded bool) {
	return c.SetIfAbsent(key, val)
}

// Size returns the total number of items (alias for Len).
// API matches Sharded.Size
func (c *Concurrent[K, V]) Size() int {
	return c.Len()
}

// ForEach iterates over all items. Return false to stop.
// Expired items are skipped and deleted.
// API matches Sharded.ForEach
func (c *Concurrent[K, V]) ForEach(fn func(K, V) bool) {
	c.Range(fn)
}

// ClearIf removes entries matching predicate and returns count removed.
// API matches Sharded.ClearIf
func (c *Concurrent[K, V]) ClearIf(shouldRemove func(K, V) bool) int {
	var total int
	c.m.Range(func(key K, entry *concurrentEntry[V]) bool {
		// Check expiration first
		if entry.expiration > 0 && nowNano() > entry.expiration {
			c.m.Delete(key)
			total++
			return true
		}

		if shouldRemove(key, entry.value) {
			c.m.Delete(key)
			total++
		}
		return true
	})
	return total
}

// Replace replaces the value for a key only if it exists.
// Returns the old value and true if replaced.
// API matches Sharded.Replace
func (c *Concurrent[K, V]) Replace(key K, val V) (V, bool) {
	var old V
	var replaced bool

	c.m.Compute(key, func(current *concurrentEntry[V], exists bool) (*concurrentEntry[V], bool) {
		if !exists || current == nil {
			return nil, false // don't create
		}
		// Check expiration
		if current.expiration > 0 && nowNano() > current.expiration {
			return nil, false // expired, don't create
		}
		old = current.value
		replaced = true
		return &concurrentEntry[V]{value: val, expiration: current.expiration}, true
	})

	return old, replaced
}

// CompareAndSwap swaps the value if the current value matches old.
// API matches Sharded.CompareAndSwap
func (c *Concurrent[K, V]) CompareAndSwap(key K, old V, newV V) bool {
	var swapped bool
	c.m.Compute(key, func(current *concurrentEntry[V], exists bool) (*concurrentEntry[V], bool) {
		if !exists || current == nil {
			swapped = false
			return nil, false
		}

		// Check expiration
		if current.expiration > 0 && nowNano() > current.expiration {
			swapped = false
			return nil, false
		}

		// Use reflection for generic comparison
		if any(current.value) != any(old) {
			swapped = false
			return current, true
		}

		swapped = true
		return &concurrentEntry[V]{value: newV, expiration: current.expiration}, true
	})
	return swapped
}

// nowNano returns current time in nanoseconds.
func nowNano() int64 {
	return time.Now().UnixNano()
}
