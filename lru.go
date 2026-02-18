package mappo

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/puzpuzpuz/xsync/v3"
)

// LRUConfig holds configuration for LRU map.
type LRUConfig[K comparable, V any] struct {
	MaxSize    int
	TTL        time.Duration
	OnEviction func(key K, value V)
}

// lruNode is an intrusive list node stored in the node pool.
type lruNode[K comparable, V any] struct {
	key        K
	value      V
	expiration int64 // UnixNano, 0 means no expiration
	prev       int64 // Index in nodePool, -1 if none
	next       int64 // Index in nodePool, -1 if none
}

// LRU provides a high-performance concurrent LRU map with optional TTL.
// Uses xsync.MapOf for lock-free lookups and sharded locks for writes.
type LRU[K comparable, V any] struct {
	maxSize    int
	defaultTTL time.Duration
	onEviction func(K, V)

	// Lock-free map stores int64 indices into nodePool
	m *xsync.MapOf[K, int64]

	// List management
	head     atomic.Int64 // Index of head node
	tail     atomic.Int64 // Index of tail node
	freeList atomic.Int64 // Index of first free node (for reuse)

	// Node pool for intrusive list - avoids allocations
	nodePool []lruNode[K, V]
	poolMu   sync.Mutex

	// Size tracking
	size atomic.Int32

	// Cleanup coordination
	cleanupMu sync.Mutex
}

// NewLRU creates a new LRU map.
func NewLRU[K comparable, V any](maxSize int) *LRU[K, V] {
	return NewLRUWithConfig[K, V](LRUConfig[K, V]{
		MaxSize: maxSize,
	})
}

// NewLRUWithConfig creates a new LRU map with configuration.
func NewLRUWithConfig[K comparable, V any](cfg LRUConfig[K, V]) *LRU[K, V] {
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = 1000
	}

	l := &LRU[K, V]{
		maxSize:    cfg.MaxSize,
		defaultTTL: cfg.TTL,
		onEviction: cfg.OnEviction,
		m:          xsync.NewMapOf[K, int64](),
		nodePool:   make([]lruNode[K, V], 0, cfg.MaxSize),
	}
	l.head.Store(-1)
	l.tail.Store(-1)
	l.freeList.Store(-1)

	return l
}

// acquireNode gets a node from pool or allocates new.
func (l *LRU[K, V]) acquireNode() int64 {
	l.poolMu.Lock()
	defer l.poolMu.Unlock()

	// Try free list first
	freeIdx := l.freeList.Load()
	if freeIdx >= 0 {
		node := &l.nodePool[freeIdx]
		l.freeList.Store(node.next)
		return freeIdx
	}

	// Allocate new
	idx := int64(len(l.nodePool))
	l.nodePool = append(l.nodePool, lruNode[K, V]{})
	return idx
}

// releaseNode returns node to free list.
func (l *LRU[K, V]) releaseNode(idx int64) {
	l.poolMu.Lock()
	defer l.poolMu.Unlock()

	node := &l.nodePool[idx]
	node.next = l.freeList.Load()
	l.freeList.Store(idx)
}

// addToFront adds node to front of list.
func (l *LRU[K, V]) addToFront(idx int64) {
	node := &l.nodePool[idx]
	node.prev = -1
	node.next = l.head.Load()

	if headIdx := l.head.Load(); headIdx >= 0 {
		l.nodePool[headIdx].prev = idx
	} else {
		// Empty list, also set tail
		l.tail.Store(idx)
	}
	l.head.Store(idx)
}

// removeFromList removes node from list.
func (l *LRU[K, V]) removeFromList(idx int64) {
	node := &l.nodePool[idx]

	if prevIdx := node.prev; prevIdx >= 0 {
		l.nodePool[prevIdx].next = node.next
	} else {
		// Was head
		l.head.Store(node.next)
	}

	if nextIdx := node.next; nextIdx >= 0 {
		l.nodePool[nextIdx].prev = node.prev
	} else {
		// Was tail
		l.tail.Store(node.prev)
	}

	node.prev = -1
	node.next = -1
}

// moveToFront moves existing node to front.
func (l *LRU[K, V]) moveToFront(idx int64) {
	l.removeFromList(idx)
	l.addToFront(idx)
}

// evictBack removes and returns the tail node.
func (l *LRU[K, V]) evictBack() (K, V, bool) {
	tailIdx := l.tail.Load()
	if tailIdx < 0 {
		var zeroK K
		var zeroV V
		return zeroK, zeroV, false
	}

	node := &l.nodePool[tailIdx]
	l.removeFromList(tailIdx)
	l.m.Delete(node.key)
	l.releaseNode(tailIdx)
	l.size.Add(-1)

	return node.key, node.value, true
}

// Set adds or updates a value with default TTL.
func (l *LRU[K, V]) Set(key K, value V) {
	l.SetWithTTL(key, value, l.defaultTTL)
}

// SetWithTTL adds or updates a value with specific TTL.
func (l *LRU[K, V]) SetWithTTL(key K, value V, ttl time.Duration) {
	var exp int64
	if ttl > 0 {
		exp = time.Now().Add(ttl).UnixNano()
	}

	// Try to update existing
	if existingIdx, ok := l.m.Load(key); ok {
		node := &l.nodePool[existingIdx]
		node.value = value
		node.expiration = exp
		l.moveToFront(existingIdx)
		return
	}

	// New entry
	idx := l.acquireNode()
	node := &l.nodePool[idx]
	node.key = key
	node.value = value
	node.expiration = exp
	node.prev = -1
	node.next = -1

	// Store in map (stores the int64 index)
	l.m.Store(key, idx)
	l.addToFront(idx)

	// Check eviction
	if l.size.Add(1) > int32(l.maxSize) {
		if l.onEviction != nil {
			k, v, _ := l.evictBack()
			l.onEviction(k, v)
		} else {
			l.evictBack()
		}
	}
}

// Get retrieves a value and updates access time.
func (l *LRU[K, V]) Get(key K) (V, bool) {
	idx, ok := l.m.Load(key)
	if !ok {
		var zero V
		return zero, false
	}

	node := &l.nodePool[idx]

	// Check expiration
	if node.expiration > 0 && time.Now().UnixNano() > node.expiration {
		l.removeFromList(idx)
		l.m.Delete(key)
		l.releaseNode(idx)
		l.size.Add(-1)
		var zero V
		return zero, false
	}

	l.moveToFront(idx)
	return node.value, true
}

// Peek returns a value without updating LRU status.
func (l *LRU[K, V]) Peek(key K) (V, bool) {
	idx, ok := l.m.Load(key)
	if !ok {
		var zero V
		return zero, false
	}

	node := &l.nodePool[idx]

	// Check expiration without modifying list
	if node.expiration > 0 && time.Now().UnixNano() > node.expiration {
		// Expired - need to remove. Use Compute for atomic check-delete
		l.m.Compute(key, func(oldIdx int64, exists bool) (int64, bool) {
			if !exists {
				return 0, false
			}
			// Double-check expiration
			n := &l.nodePool[oldIdx]
			if n.expiration > 0 && time.Now().UnixNano() > n.expiration {
				l.cleanupMu.Lock()
				l.removeFromList(oldIdx)
				l.releaseNode(oldIdx)
				l.cleanupMu.Unlock()
				l.size.Add(-1)
				return 0, false
			}
			return oldIdx, true
		})
		var zero V
		return zero, false
	}

	return node.value, true
}

// Delete removes a key.
func (l *LRU[K, V]) Delete(key K) bool {
	idx, ok := l.m.Load(key)
	if !ok {
		return false
	}

	l.m.Delete(key)
	l.cleanupMu.Lock()
	l.removeFromList(idx)
	l.releaseNode(idx)
	l.cleanupMu.Unlock()
	l.size.Add(-1)
	return true
}

// Has returns true if the key exists and is not expired.
func (l *LRU[K, V]) Has(key K) bool {
	_, ok := l.Peek(key)
	return ok
}

// Len returns the number of items.
func (l *LRU[K, V]) Len() int {
	return int(l.size.Load())
}

// Clear removes all items.
func (l *LRU[K, V]) Clear() {
	l.cleanupMu.Lock()
	defer l.cleanupMu.Unlock()

	if l.onEviction != nil {
		l.m.Range(func(key K, idx int64) bool {
			node := &l.nodePool[idx]
			l.onEviction(node.key, node.value)
			return true
		})
	}

	l.m.Clear()
	l.nodePool = l.nodePool[:0]
	l.head.Store(-1)
	l.tail.Store(-1)
	l.freeList.Store(-1)
	l.size.Store(0)
}

// Keys returns all keys in order from most to least recent.
func (l *LRU[K, V]) Keys() []K {
	l.cleanupMu.Lock()
	defer l.cleanupMu.Unlock()

	keys := make([]K, 0, l.Len())
	now := time.Now().UnixNano()

	for idx := l.head.Load(); idx >= 0; {
		node := &l.nodePool[idx]
		if node.expiration == 0 || node.expiration > now {
			keys = append(keys, node.key)
		}
		idx = node.next
	}
	return keys
}

// Values returns all values in order from most to least recent.
func (l *LRU[K, V]) Values() []V {
	l.cleanupMu.Lock()
	defer l.cleanupMu.Unlock()

	values := make([]V, 0, l.Len())
	now := time.Now().UnixNano()

	for idx := l.head.Load(); idx >= 0; {
		node := &l.nodePool[idx]
		if node.expiration == 0 || node.expiration > now {
			values = append(values, node.value)
		}
		idx = node.next
	}
	return values
}

// ForEach iterates over items from most to least recent.
func (l *LRU[K, V]) ForEach(fn func(K, V) bool) {
	l.cleanupMu.Lock()
	defer l.cleanupMu.Unlock()

	now := time.Now().UnixNano()
	for idx := l.head.Load(); idx >= 0; {
		node := &l.nodePool[idx]
		nextIdx := node.next // Save before potential modification

		if node.expiration == 0 || node.expiration > now {
			if !fn(node.key, node.value) {
				return
			}
		}
		idx = nextIdx
	}
}

// GetOrSet returns the existing value for the key if present and not expired,
// otherwise sets and returns the given value.
func (l *LRU[K, V]) GetOrSet(key K, value V, ttl time.Duration) (V, bool) {
	// Fast path
	if v, ok := l.Get(key); ok {
		return v, true
	}

	// Slow path with proper TTL handling
	var exp int64
	if ttl > 0 {
		exp = time.Now().Add(ttl).UnixNano()
	}

	// Use Compute for atomicity
	var result V
	var loaded bool

	l.m.Compute(key, func(oldIdx int64, exists bool) (int64, bool) {
		if exists {
			node := &l.nodePool[oldIdx]
			// Check expiration
			if node.expiration == 0 || time.Now().UnixNano() <= node.expiration {
				result = node.value
				loaded = true
				l.moveToFront(oldIdx)
				return oldIdx, true
			}
			// Expired, remove old
			l.removeFromList(oldIdx)
			l.releaseNode(oldIdx)
			l.size.Add(-1)
		}

		// Insert new
		idx := l.acquireNode()
		node := &l.nodePool[idx]
		node.key = key
		node.value = value
		node.expiration = exp
		node.prev = -1
		node.next = -1

		l.addToFront(idx)
		if l.size.Add(1) > int32(l.maxSize) {
			if l.onEviction != nil {
				k, v, _ := l.evictBack()
				l.onEviction(k, v)
			} else {
				l.evictBack()
			}
		}

		result = value
		loaded = false
		return idx, true
	})

	return result, loaded
}

// GetOrCompute returns the existing value or computes and stores a new one.
func (l *LRU[K, V]) GetOrCompute(key K, fn func() (V, time.Duration)) V {
	// Fast path
	if v, ok := l.Get(key); ok {
		return v
	}

	// Compute value
	val, ttl := fn()
	actual, _ := l.GetOrSet(key, val, ttl)
	return actual
}

// Resize changes the maximum size and evicts if necessary.
func (l *LRU[K, V]) Resize(maxSize int) {
	if maxSize <= 0 {
		maxSize = 1000
	}

	l.cleanupMu.Lock()
	defer l.cleanupMu.Unlock()

	l.maxSize = maxSize

	// Evict excess
	for int(l.size.Load()) > maxSize {
		if l.onEviction != nil {
			k, v, _ := l.evictBack()
			l.onEviction(k, v)
		} else {
			l.evictBack()
		}
	}
}

// PurgeExpired removes all expired items and returns count removed.
func (l *LRU[K, V]) PurgeExpired() int {
	l.cleanupMu.Lock()
	defer l.cleanupMu.Unlock()

	now := time.Now().UnixNano()
	removed := 0

	// Iterate through list and remove expired
	for idx := l.head.Load(); idx >= 0; {
		node := &l.nodePool[idx]
		nextIdx := node.next

		if node.expiration > 0 && now > node.expiration {
			l.m.Delete(node.key)
			l.removeFromList(idx)
			l.releaseNode(idx)
			l.size.Add(-1)
			if l.onEviction != nil {
				l.onEviction(node.key, node.value)
			}
			removed++
		}
		idx = nextIdx
	}

	return removed
}
