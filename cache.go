// cache.go
package mappo

import (
	"encoding/json"
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
	inner *otter.Cache[string, *Item]
	now   func() time.Time
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

// Load retrieves an item. Returns false if key doesn't exist or is expired.
func (c *Cache) Load(key string) (*Item, bool) {
	it, ok := c.inner.GetIfPresent(key)
	if !ok || it == nil {
		return nil, false
	}
	if !it.Exp.IsZero() && c.now().After(it.Exp) {
		c.inner.Invalidate(key)
		return nil, false
	}
	return it, true
}

// Store stores an item.
func (c *Cache) Store(key string, it *Item) {
	c.inner.Set(key, it)
}

// StoreTTL stores an item with TTL.
func (c *Cache) StoreTTL(key string, it *Item, ttl time.Duration) {
	if it == nil {
		return
	}
	if ttl > 0 {
		it.Exp = c.now().Add(ttl)
	} else {
		it.Exp = time.Time{}
	}
	c.inner.Set(key, it)
}

// LoadOrStore loads or stores an item.
func (c *Cache) LoadOrStore(key string, it *Item) (*Item, bool) {
	v, stored := c.inner.SetIfAbsent(key, it)
	if stored {
		return v, false
	}

	if v == nil {
		return nil, true
	}

	if !v.Exp.IsZero() && c.now().After(v.Exp) {
		c.inner.Invalidate(key)
		c.inner.Set(key, it)
		return it, false
	}

	return v, true
}

// Delete removes a key.
func (c *Cache) Delete(key string) {
	c.inner.Invalidate(key)
}

// LoadAndDelete loads and deletes an item.
func (c *Cache) LoadAndDelete(key string) (*Item, bool) {
	it, ok := c.Load(key)
	if !ok {
		return nil, false
	}
	c.inner.Invalidate(key)
	return it, true
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
// otherwise sets and returns the given value.
func (c *Cache) GetOrSet(key string, value any, ttl time.Duration) (any, bool) {
	// Try to get existing
	if existing, ok := c.Load(key); ok {
		return existing.Value, true
	}

	// Set new value
	it := &Item{
		Value: value,
	}
	if ttl > 0 {
		it.Exp = c.now().Add(ttl)
	}
	c.inner.Set(key, it)
	return value, false
}

// GetOrCompute returns the existing value or computes and stores a new one.
func (c *Cache) GetOrCompute(key string, fn func() (any, time.Duration)) any {
	// Try to get existing
	if existing, ok := c.Load(key); ok {
		return existing.Value
	}

	// Compute new value
	val, ttl := fn()
	it := &Item{
		Value: val,
	}
	if ttl > 0 {
		it.Exp = c.now().Add(ttl)
	}
	c.inner.Set(key, it)
	return val
}

// Has returns true if the key exists and is not expired.
func (c *Cache) Has(key string) bool {
	_, ok := c.Load(key)
	return ok
}

// Len returns the number of items in the cache.
func (c *Cache) Len() int {
	return c.inner.EstimatedSize()
}

// Clear removes all items.
func (c *Cache) Clear() {
	c.inner.InvalidateAll()
}

// Range iterates over all items in the cache.
// Return false to stop iteration.
func (c *Cache) Range(fn func(key string, item *Item) bool) {
	c.inner.All()(func(key string, item *Item) bool {
		// Check expiration
		if !item.Exp.IsZero() && c.now().After(item.Exp) {
			c.inner.Invalidate(key)
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
	stats := c.inner.Stats()
	return CacheStats{
		Hits:      int64(stats.Hits),
		Misses:    int64(stats.Misses),
		Evictions: int64(stats.Evictions),
		Size:      int64(c.Len()),
		Capacity:  int64(c.inner.GetMaximum()),
	}
}
