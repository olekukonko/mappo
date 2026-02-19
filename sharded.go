package mappo

import (
	"hash/maphash"
	"math/bits"
	"reflect"
	"runtime"

	"github.com/puzpuzpuz/xsync/v3"
)

// cacheLineSize is the assumed size of a CPU cache line.
// Comment out padding if testing without it.
const cacheLineSize = 64

// padding ensures shard doesn't false share with adjacent shards.
type padding [cacheLineSize]byte

// shard holds a portion of the map with its own lock-free structure.
type shard[K comparable, V any] struct {
	_    padding
	data *xsync.MapOf[K, V]
	_    padding
}

// Sharded provides a generic sharded map for high-concurrency scenarios.
// It reduces lock contention by splitting the map into multiple shards.
type Sharded[K comparable, V any] struct {
	shards []shard[K, V]
	mask   uint64
	seed   maphash.Seed
	hash   func(K, maphash.Seed) uint64
}

// ShardedConfig holds configuration for Sharded map.
type ShardedConfig struct {
	// ShardCount is the number of shards (rounded up to power of 2).
	// If <= 0, defaults to NumCPU.
	ShardCount int
}

// DefaultShardedConfig returns default configuration.
func DefaultShardedConfig() ShardedConfig {
	return ShardedConfig{
		ShardCount: runtime.NumCPU(),
	}
}

// NewSharded creates a new sharded map with default configuration.
func NewSharded[K comparable, V any]() *Sharded[K, V] {
	return NewShardedWithConfig[K, V](DefaultShardedConfig())
}

// NewShardedWithConfig creates a new sharded map with custom configuration.
func NewShardedWithConfig[K comparable, V any](cfg ShardedConfig) *Sharded[K, V] {
	shardCount := cfg.ShardCount
	if shardCount <= 0 {
		shardCount = runtime.NumCPU()
	}

	// Round up to next power of 2
	n := shardCount
	if n <= 0 {
		n = 2
	}
	n = 1 << bits.Len64(uint64(n)-1)
	if n < 2 {
		n = 2
	}
	shardCount = n

	sm := &Sharded[K, V]{
		shards: make([]shard[K, V], shardCount),
		mask:   uint64(shardCount - 1),
		seed:   maphash.MakeSeed(),
		hash:   makeHasher[K](),
	}

	for i := range sm.shards {
		sm.shards[i].data = xsync.NewMapOf[K, V]()
	}

	return sm
}

func (sm *Sharded[K, V]) shardIndex(key K) int {
	h := sm.hash(key, sm.seed)
	return int(h & sm.mask)
}

func (sm *Sharded[K, V]) getShard(key K) *shard[K, V] {
	return &sm.shards[sm.shardIndex(key)]
}

// Get retrieves a value. Safe for concurrent use.
func (sm *Sharded[K, V]) Get(key K) (V, bool) {
	shard := sm.getShard(key)
	return shard.data.Load(key)
}

// Set sets a value. Safe for concurrent use.
func (sm *Sharded[K, V]) Set(key K, val V) {
	shard := sm.getShard(key)
	shard.data.Store(key, val)
}

// SetIfAbsent sets the value only if the key doesn't exist.
// Returns the actual value and true if loaded (already existed).
func (sm *Sharded[K, V]) SetIfAbsent(key K, val V) (V, bool) {
	shard := sm.getShard(key)

	var actual V
	var loaded bool

	shard.data.Compute(key, func(oldV V, exists bool) (V, bool) {
		if exists {
			actual = oldV
			loaded = true
			return oldV, false // delete=false, keep existing
		}
		actual = val
		loaded = false
		return val, false // delete=false, store new
	})

	return actual, loaded
}

// Update performs an atomic read-modify-write and returns the new value.
// Semantically equivalent to Compute(fn) but signals "always keep" intent.
// API matches Concurrent.Update
func (sm *Sharded[K, V]) Update(key K, fn func(current V, exists bool) V) V {
	return sm.Compute(key, func(curr V, exists bool) (V, bool) {
		return fn(curr, exists), true
	})
}

// Compute allows atomic read-modify-write operations on a key within a shard.
// The function fn receives the current value (or zero value) and existence flag.
// It returns the new value and a boolean indicating if the key should be kept (true) or deleted (false).
func (sm *Sharded[K, V]) Compute(key K, fn func(current V, exists bool) (newValue V, keep bool)) V {
	shard := sm.getShard(key)

	var result V
	shard.data.Compute(key, func(oldV V, exists bool) (V, bool) {
		newV, keep := fn(oldV, exists)
		if keep {
			result = newV
			return newV, false // delete=false
		}
		// Delete
		var zero V
		result = zero
		return zero, true // delete=true
	})

	return result
}

// Replace replaces the value for a key only if it exists.
// Returns the old value and true if replaced.
func (sm *Sharded[K, V]) Replace(key K, val V) (V, bool) {
	shard := sm.getShard(key)

	var old V
	var replaced bool

	shard.data.Compute(key, func(current V, exists bool) (V, bool) {
		if !exists {
			var zero V
			return zero, true // delete=true, no create
		}
		old = current
		replaced = true
		return val, false // delete=false
	})

	return old, replaced
}

// CompareAndSwap swaps the value if the current value matches old.
func (sm *Sharded[K, V]) CompareAndSwap(key K, old V, newV V) bool {
	shard := sm.getShard(key)
	var swapped bool
	shard.data.Compute(key, func(current V, exists bool) (V, bool) {
		if !exists {
			swapped = false
			var zero V
			return zero, true // delete=true, no store
		}

		// Fast path: direct comparison via any() for comparable types
		// This avoids reflection overhead for primitives, strings, etc.
		if any(current) == any(old) {
			swapped = true
			return newV, false // delete=false, store
		}

		// Slow path: use reflection for complex types
		if !reflect.DeepEqual(current, old) {
			swapped = false
			return current, false // delete=false, keep
		}

		swapped = true
		return newV, false // delete=false, store
	})
	return swapped
}

// Delete removes a key. Safe for concurrent use.
func (sm *Sharded[K, V]) Delete(key K) bool {
	shard := sm.getShard(key)
	existed := false
	var zero V
	shard.data.Compute(key, func(current V, found bool) (V, bool) {
		if found {
			existed = true
		}
		return zero, true // delete=true
	})
	return existed
}

// Clear resets all shards.
func (sm *Sharded[K, V]) Clear() {
	for i := range sm.shards {
		sm.shards[i].data.Clear()
	}
}

// ClearIf removes entries matching predicate and returns count removed.
func (sm *Sharded[K, V]) ClearIf(shouldRemove func(K, V) bool) int {
	var total int64
	for i := range sm.shards {
		shard := &sm.shards[i]
		var toDelete []K
		shard.data.Range(func(k K, v V) bool {
			if shouldRemove(k, v) {
				toDelete = append(toDelete, k)
			}
			return true
		})
		for _, k := range toDelete {
			shard.data.Delete(k)
			total++
		}
	}
	return int(total)
}

// Len returns the total number of items across all shards.
func (sm *Sharded[K, V]) Len() int {
	var total int
	for i := range sm.shards {
		total += sm.shards[i].data.Size()
	}
	return total
}

// Size returns the total number of items (alias for Len).
func (sm *Sharded[K, V]) Size() int {
	return sm.Len()
}

// ShardStats returns the distribution of items per shard.
func (sm *Sharded[K, V]) ShardStats() []int {
	stats := make([]int, len(sm.shards))
	for i := range sm.shards {
		stats[i] = sm.shards[i].data.Size()
	}
	return stats
}

// ForEach iterates through all items. Return false to stop iteration.
// API matches Concurrent.ForEach
func (sm *Sharded[K, V]) ForEach(fn func(K, V) bool) {
	for i := range sm.shards {
		cont := true
		sm.shards[i].data.Range(func(k K, v V) bool {
			cont = fn(k, v)
			return cont
		})
		if !cont {
			return
		}
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
	return sm.SetIfAbsent(key, val)
}
