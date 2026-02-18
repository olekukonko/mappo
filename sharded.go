// sharded.go
package mappo

import (
	"fmt"
	"math/bits"
	"sync"

	"github.com/cespare/xxhash/v2"
)

// Sharded provides a generic sharded map for high-concurrency scenarios.
// It reduces lock contention by splitting the map into multiple shards.
// Key K must be comparable; user provides hash func for sharding.
type Sharded[K comparable, V any] struct {
	shards []shard[K, V]
	mask   uint32
	hash   func(K) uint32
}

type shard[K comparable, V any] struct {
	mu   sync.RWMutex
	data map[K]V
}

// ShardedConfig holds configuration for Sharded map.
type ShardedConfig struct {
	// ShardCount is the number of shards (rounded up to power of 2)
	ShardCount int
	// HashFunc is optional custom hash function (uses xxhash if nil)
	HashFunc func(key any) uint32
}

// DefaultShardedConfig returns default configuration.
func DefaultShardedConfig() ShardedConfig {
	return ShardedConfig{
		ShardCount: 64,
		HashFunc:   nil,
	}
}

// NewSharded creates a new sharded map with default configuration.
func NewSharded[K comparable, V any]() *Sharded[K, V] {
	return NewShardedWithConfig[K, V](DefaultShardedConfig())
}

// NewShardedWithConfig creates a new sharded map with custom configuration.
func NewShardedWithConfig[K comparable, V any](cfg ShardedConfig) *Sharded[K, V] {
	if cfg.ShardCount <= 0 {
		cfg.ShardCount = 64
	}
	// Round up to power of 2 for better distribution
	shardCount := 1 << bits.Len(uint(cfg.ShardCount-1))

	hashFunc := cfg.HashFunc
	if hashFunc == nil {
		// Default to xxhash for any key type
		hashFunc = func(key any) uint32 {
			h64 := xxhash.Sum64String(fmt.Sprint(key))
			return uint32(h64 ^ (h64 >> 32))
		}
	}

	typedHash := func(k K) uint32 {
		return hashFunc(any(k))
	}

	sm := &Sharded[K, V]{
		shards: make([]shard[K, V], shardCount),
		mask:   uint32(shardCount - 1),
		hash:   typedHash,
	}
	for i := range sm.shards {
		sm.shards[i].data = make(map[K]V)
	}
	return sm
}

func (sm *Sharded[K, V]) shardIndex(key K) int {
	return int(sm.hash(key) & sm.mask)
}

// Get retrieves a value. Safe for concurrent use.
func (sm *Sharded[K, V]) Get(key K) (V, bool) {
	idx := sm.shardIndex(key)
	shard := &sm.shards[idx]
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	v, ok := shard.data[key]
	return v, ok
}

// Set sets a value. Safe for concurrent use.
func (sm *Sharded[K, V]) Set(key K, val V) {
	idx := sm.shardIndex(key)
	shard := &sm.shards[idx]
	shard.mu.Lock()
	shard.data[key] = val
	shard.mu.Unlock()
}

// Compute allows atomic read-modify-write operations on a key within a shard lock.
// The function fn receives the current value (or zero value) and existence flag.
// It returns the new value and a boolean indicating if the key should be kept (true) or deleted (false).
func (sm *Sharded[K, V]) Compute(key K, fn func(current V, exists bool) (newValue V, keep bool)) V {
	idx := sm.shardIndex(key)
	shard := &sm.shards[idx]
	shard.mu.Lock()
	defer shard.mu.Unlock()

	curr, exists := shard.data[key]
	newVal, keep := fn(curr, exists)

	if keep {
		shard.data[key] = newVal
	} else {
		delete(shard.data, key)
	}
	return newVal
}

// Delete removes a key. Safe for concurrent use.
func (sm *Sharded[K, V]) Delete(key K) {
	idx := sm.shardIndex(key)
	shard := &sm.shards[idx]
	shard.mu.Lock()
	delete(shard.data, key)
	shard.mu.Unlock()
}

// Clear resets all shards.
func (sm *Sharded[K, V]) Clear() {
	for i := range sm.shards {
		shard := &sm.shards[i]
		shard.mu.Lock()
		shard.data = make(map[K]V)
		shard.mu.Unlock()
	}
}

// ClearIf removes entries matching predicate and returns count removed.
func (sm *Sharded[K, V]) ClearIf(shouldRemove func(K, V) bool) int {
	var removed int
	for i := range sm.shards {
		shard := &sm.shards[i]
		shard.mu.Lock()
		for k, v := range shard.data {
			if shouldRemove(k, v) {
				delete(shard.data, k)
				removed++
			}
		}
		shard.mu.Unlock()
	}
	return removed
}

// Len returns the total number of items across all shards.
func (sm *Sharded[K, V]) Len() int {
	count := 0
	for i := range sm.shards {
		shard := &sm.shards[i]
		shard.mu.RLock()
		count += len(shard.data)
		shard.mu.RUnlock()
	}
	return count
}

// ForEach iterates through all items. Return false to stop iteration.
func (sm *Sharded[K, V]) ForEach(fn func(K, V) bool) {
	for i := range sm.shards {
		shard := &sm.shards[i]
		shard.mu.RLock()
		for k, v := range shard.data {
			if !fn(k, v) {
				shard.mu.RUnlock()
				return
			}
		}
		shard.mu.RUnlock()
	}
}

// Keys returns all keys in the map.
func (sm *Sharded[K, V]) Keys() []K {
	keys := make([]K, 0, sm.Len())
	sm.ForEach(func(k K, _ V) bool {
		keys = append(keys, k)
		return true
	})
	return keys
}

// Values returns all values in the map.
func (sm *Sharded[K, V]) Values() []V {
	values := make([]V, 0, sm.Len())
	sm.ForEach(func(_ K, v V) bool {
		values = append(values, v)
		return true
	})
	return values
}

// Has returns true if the key exists.
func (sm *Sharded[K, V]) Has(key K) bool {
	_, ok := sm.Get(key)
	return ok
}

// GetOrSet returns the existing value for the key if present, otherwise sets and returns the given value.
func (sm *Sharded[K, V]) GetOrSet(key K, val V) (actual V, loaded bool) {
	idx := sm.shardIndex(key)
	shard := &sm.shards[idx]
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if existing, ok := shard.data[key]; ok {
		return existing, true
	}
	shard.data[key] = val
	return val, false
}
