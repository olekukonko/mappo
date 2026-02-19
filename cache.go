package mappo

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/maypok86/otter/v2"
	"github.com/maypok86/otter/v2/stats"
)

// Item represents a cached item with expiration and access tracking.
type Item struct {
	Value        any          `json:"value"`
	LastAccessed atomic.Int64 `json:"last_accessed"`
	Exp          time.Time    `json:"exp"`
}

// MarshalJSON implements json.Marshaler.
func (it *Item) MarshalJSON() ([]byte, error) {
	type Alias Item
	return json.Marshal(&struct {
		LastAccessed int64 `json:"last_accessed"`
		*Alias
	}{
		LastAccessed: it.LastAccessed.Load(),
		Alias:        (*Alias)(it),
	})
}

// UnmarshalJSON implements json.Unmarshaler.
func (it *Item) UnmarshalJSON(b []byte) error {
	type Alias Item
	aux := &struct {
		LastAccessed int64 `json:"last_accessed"`
		*Alias
	}{
		Alias: (*Alias)(it),
	}
	if err := json.Unmarshal(b, aux); err != nil {
		return err
	}
	it.LastAccessed.Store(aux.LastAccessed)
	return nil
}

// CacheOptions holds configuration for Cache.
type CacheOptions struct {
	MaximumSize int
	OnDelete    func(key string, it *Item)
	Now         func() time.Time
}

// Cache provides a high-performance concurrent cache with TTL support.
// It uses Otter as the underlying cache for optimal performance.
type Cache struct {
	inner  *otter.Cache[string, *Item]
	now    func() time.Time
	closed atomic.Bool
	mu     sync.RWMutex
}

// NewCache creates a new Cache with the given options.
func NewCache(opt CacheOptions) *Cache {
	nowFn := opt.Now
	if nowFn == nil {
		nowFn = time.Now
	}

	counter := stats.NewCounter()

	c := otter.Must(&otter.Options[string, *Item]{
		MaximumSize:   opt.MaximumSize,
		StatsRecorder: counter,
		OnDeletion: func(e otter.DeletionEvent[string, *Item]) {
			if opt.OnDelete == nil || e.Value == nil {
				return
			}
			opt.OnDelete(e.Key, e.Value)
		},
	})

	return &Cache{inner: c, now: nowFn}
}

// nowTime returns current time, using custom function if set.
func (c *Cache) nowTime() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// Load retrieves an item. Returns false if key doesn't exist or is expired.
func (c *Cache) Load(key string) (*Item, bool) {
	if c.closed.Load() {
		return nil, false
	}
	it, ok := c.inner.GetIfPresent(key)
	if !ok || it == nil {
		return nil, false
	}
	now := c.nowTime()
	if !it.Exp.IsZero() && now.After(it.Exp) {
		c.inner.Invalidate(key)
		return nil, false
	}
	it.LastAccessed.Store(now.UnixNano())
	return it, true
}

// Store stores an item.
func (c *Cache) Store(key string, it *Item) {
	if c.closed.Load() || it == nil {
		return
	}
	c.inner.Set(key, it)
}

// StoreTTL stores an item with TTL.
func (c *Cache) StoreTTL(key string, it *Item, ttl time.Duration) {
	if c.closed.Load() || it == nil {
		return
	}
	if ttl > 0 {
		it.Exp = c.nowTime().Add(ttl)
	} else {
		it.Exp = time.Time{}
	}
	c.inner.Set(key, it)
}

// LoadOrStore loads or stores an item atomically.
// Returns the actual value stored and true if the value was loaded (already existed), false if stored.
func (c *Cache) LoadOrStore(key string, it *Item) (*Item, bool) {
	if c.closed.Load() || it == nil {
		return nil, false
	}

	// Try to store if absent
	v, stored := c.inner.SetIfAbsent(key, it)
	if stored {
		// We stored it successfully
		return it, false
	}

	// Key already exists, check expiration
	if v == nil {
		// Inconsistent state, overwrite
		c.inner.Set(key, it)
		return it, false
	}

	now := c.nowTime()
	if !v.Exp.IsZero() && now.After(v.Exp) {
		// Expired, replace using Compute
		actual, _ := c.inner.Compute(key, func(current *Item, found bool) (*Item, otter.ComputeOp) {
			if !found {
				return it, otter.WriteOp
			}
			// Check again under lock
			if current != nil && !current.Exp.IsZero() && now.After(current.Exp) {
				return it, otter.WriteOp
			}
			return current, otter.CancelOp
		})
		if actual == it {
			return it, false
		}
		return actual, true
	}

	return v, true
}

// Delete removes a key.
func (c *Cache) Delete(key string) {
	if c.closed.Load() {
		return
	}
	c.inner.Invalidate(key)
}

// LoadAndDelete loads and deletes an item atomically.
func (c *Cache) LoadAndDelete(key string) (*Item, bool) {
	if c.closed.Load() {
		return nil, false
	}

	// Use Compute to get atomic read-delete
	var deleted *Item
	c.inner.Compute(key, func(current *Item, found bool) (*Item, otter.ComputeOp) {
		if !found || current == nil {
			return nil, otter.CancelOp
		}
		// Check expiration
		now := c.nowTime()
		if !current.Exp.IsZero() && now.After(current.Exp) {
			deleted = nil
			return nil, otter.InvalidateOp // Delete expired
		}
		deleted = current
		return nil, otter.InvalidateOp // Delete
	})

	if deleted == nil {
		return nil, false
	}
	return deleted, true
}

// GetTyped retrieves and type-asserts a value.
func GetTyped[T any](it *Item) (T, bool) {
	var zero T
	if it == nil || it.Value == nil {
		return zero, false
	}
	v, ok := it.Value.(T)
	return v, ok
}

// GetValue retrieves the value directly. Returns false if key doesn't exist or is expired.
func (c *Cache) GetValue(key string) (any, bool) {
	it, ok := c.Load(key)
	if !ok {
		return nil, false
	}
	return it.Value, true
}

// GetOrSet returns the existing value for the key if present and not expired,
// otherwise sets and returns the given value. Not fully atomic - use LoadOrStore for atomicity.
func (c *Cache) GetOrSet(key string, value any, ttl time.Duration) (any, bool) {
	// Fast path: try to get existing
	if existing, ok := c.Load(key); ok {
		return existing.Value, true
	}

	// Slow path: compute atomically
	it := &Item{
		Value: value,
	}
	if ttl > 0 {
		it.Exp = c.nowTime().Add(ttl)
	}

	actual, loaded := c.LoadOrStore(key, it)
	if loaded {
		return actual.Value, true
	}
	return value, false
}

// GetOrCompute returns the existing value or computes and stores a new one atomically.
func (c *Cache) GetOrCompute(key string, fn func() (any, time.Duration)) any {
	if c.closed.Load() {
		return nil
	}

	// Try fast path first
	if existing, ok := c.Load(key); ok {
		return existing.Value
	}

	// Use Compute for atomic operation
	var result any
	now := c.nowTime()
	c.inner.Compute(key, func(current *Item, found bool) (*Item, otter.ComputeOp) {
		if found && current != nil {
			// Check expiration
			if current.Exp.IsZero() || now.Before(current.Exp) {
				result = current.Value
				return current, otter.CancelOp
			}
		}

		// Compute new value
		val, ttl := fn()
		it := &Item{
			Value: val,
		}
		if ttl > 0 {
			it.Exp = now.Add(ttl)
		}
		result = val
		return it, otter.WriteOp
	})

	return result
}

// Has returns true if the key exists and is not expired.
func (c *Cache) Has(key string) bool {
	_, ok := c.Load(key)
	return ok
}

// Len returns the number of items in the cache.
func (c *Cache) Len() int {
	if c.closed.Load() {
		return 0
	}
	return c.inner.EstimatedSize()
}

// Clear removes all items.
func (c *Cache) Clear() {
	if c.closed.Load() {
		return
	}
	c.inner.InvalidateAll()
}

// Range iterates over all items in the cache.
// Return false to stop iteration.
// Expired items are skipped but not deleted during iteration.
func (c *Cache) Range(fn func(key string, item *Item) bool) {
	if c.closed.Load() {
		return
	}
	now := c.nowTime()
	c.inner.All()(func(key string, item *Item) bool {
		// Skip expired items without deleting (let Otter handle cleanup)
		if !item.Exp.IsZero() && now.After(item.Exp) {
			return true
		}
		return fn(key, item)
	})
}

// Keys returns all keys in the cache.
func (c *Cache) Keys() []string {
	keys := make([]string, 0, c.Len())
	c.Range(func(key string, _ *Item) bool {
		keys = append(keys, key)
		return true
	})
	return keys
}

// RefreshTTL updates the TTL of an existing item without changing its value.
// Returns true if the item was found and updated.
func (c *Cache) RefreshTTL(key string, ttl time.Duration) bool {
	if c.closed.Load() {
		return false
	}

	updated := false
	now := c.nowTime()
	c.inner.Compute(key, func(current *Item, found bool) (*Item, otter.ComputeOp) {
		if !found || current == nil {
			return nil, otter.CancelOp
		}
		// Check if expired
		if !current.Exp.IsZero() && now.After(current.Exp) {
			return nil, otter.InvalidateOp // Delete expired
		}

		if ttl > 0 {
			current.Exp = now.Add(ttl)
		} else {
			current.Exp = time.Time{}
		}
		updated = true
		return current, otter.WriteOp
	})

	return updated
}

// Touch updates the LastAccessed timestamp without fetching the full value.
// Returns true if the item exists and is not expired.
func (c *Cache) Touch(key string) bool {
	if c.closed.Load() {
		return false
	}

	touched := false
	now := c.nowTime()
	c.inner.Compute(key, func(current *Item, found bool) (*Item, otter.ComputeOp) {
		if !found || current == nil {
			return nil, otter.CancelOp
		}
		if !current.Exp.IsZero() && now.After(current.Exp) {
			return nil, otter.InvalidateOp
		}
		current.LastAccessed.Store(now.UnixNano())
		touched = true
		return current, otter.WriteOp
	})

	return touched
}

// CacheStats holds cache statistics.
type CacheStats struct {
	Hits      int64
	Misses    int64
	Evictions int64
	Size      int64
	Capacity  int64
}

// Stats returns cache statistics.
func (c *Cache) Stats() CacheStats {
	if c.closed.Load() {
		return CacheStats{}
	}
	stats := c.inner.Stats()
	return CacheStats{
		Hits:      int64(stats.Hits),
		Misses:    int64(stats.Misses),
		Evictions: int64(stats.Evictions),
		Size:      int64(c.Len()),
		Capacity:  int64(c.inner.GetMaximum()),
	}
}

// Close closes the cache and releases resources.
// Note: otter cache doesn't have an explicit Close method,
// we just mark it as closed to prevent further operations.
func (c *Cache) Close() error {
	if c.closed.CompareAndSwap(false, true) {
		c.inner.InvalidateAll()
	}
	return nil
}
