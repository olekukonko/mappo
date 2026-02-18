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
type LRU[K comparable, V any] struct {
	maxSize    int
	defaultTTL time.Duration
	onEviction func(K, V)
	m          *xsync.MapOf[K, int64]
	listMu     sync.Mutex
	head       int64
	tail       int64
	freeList   int64
	nodePool   []lruNode[K, V]
	size       atomic.Int32
}

// NewLRU creates a new LRU map.
func NewLRU[K comparable, V any](maxSize int) *LRU[K, V] {
	return NewLRUWithConfig[K, V](LRUConfig[K, V]{MaxSize: maxSize})
}

// NewLRUWithConfig creates a new LRU map with configuration.
func NewLRUWithConfig[K comparable, V any](cfg LRUConfig[K, V]) *LRU[K, V] {
	if cfg.MaxSize <= 0 {
		cfg.MaxSize = 1000
	}
	return &LRU[K, V]{
		maxSize:    cfg.MaxSize,
		defaultTTL: cfg.TTL,
		onEviction: cfg.OnEviction,
		m:          xsync.NewMapOf[K, int64](),
		nodePool:   make([]lruNode[K, V], 0, cfg.MaxSize),
		head:       -1,
		tail:       -1,
		freeList:   -1,
	}
}

func (l *LRU[K, V]) acquireNode() int64 {
	// Try free list first
	if l.freeList >= 0 {
		idx := l.freeList
		node := &l.nodePool[idx]
		l.freeList = node.next
		node.prev, node.next = -1, -1
		node.expiration = 0
		return idx
	}

	idx := int64(len(l.nodePool))
	if int(idx) >= cap(l.nodePool) {
		return -1 // Signal to evict
	}
	l.nodePool = append(l.nodePool, lruNode[K, V]{prev: -1, next: -1})
	return idx
}

func (l *LRU[K, V]) releaseNode(idx int64) {
	if idx < 0 || idx >= int64(len(l.nodePool)) {
		return
	}
	node := &l.nodePool[idx]
	var zeroK K
	var zeroV V
	node.key, node.value = zeroK, zeroV
	node.expiration = 0
	node.prev = -1
	node.next = l.freeList
	l.freeList = idx
}

func (l *LRU[K, V]) addToFront(idx int64) {
	node := &l.nodePool[idx]
	node.prev, node.next = -1, l.head
	if l.head >= 0 {
		l.nodePool[l.head].prev = idx
	} else {
		l.tail = idx
	}
	l.head = idx
}

func (l *LRU[K, V]) removeFromList(idx int64) {
	node := &l.nodePool[idx]
	if node.prev >= 0 {
		l.nodePool[node.prev].next = node.next
	} else {
		l.head = node.next
	}
	if node.next >= 0 {
		l.nodePool[node.next].prev = node.prev
	} else {
		l.tail = node.prev
	}
	node.prev, node.next = -1, -1
}

func (l *LRU[K, V]) moveToFront(idx int64) {
	l.removeFromList(idx)
	l.addToFront(idx)
}

func (l *LRU[K, V]) evictBack() (K, V, bool) {
	if l.tail < 0 {
		var zeroK K
		var zeroV V
		return zeroK, zeroV, false
	}
	idx := l.tail
	node := &l.nodePool[idx]
	l.removeFromList(idx)
	l.m.Delete(node.key)
	l.releaseNode(idx)
	l.size.Add(-1)
	return node.key, node.value, true
}

func (l *LRU[K, V]) Set(key K, value V) {
	l.SetWithTTL(key, value, l.defaultTTL)
}

func (l *LRU[K, V]) SetWithTTL(key K, value V, ttl time.Duration) {
	var exp int64
	if ttl > 0 {
		exp = time.Now().Add(ttl).UnixNano()
	}

	l.listMu.Lock()
	defer l.listMu.Unlock()

	// Update existing
	if idx, ok := l.m.Load(key); ok && idx >= 0 && idx < int64(len(l.nodePool)) {
		node := &l.nodePool[idx]
		if node.key == key {
			node.value = value
			node.expiration = exp
			l.moveToFront(idx)
			return
		}
	}

	// Evict if at capacity BEFORE acquiring new node
	for int(l.size.Load()) >= l.maxSize {
		if l.onEviction != nil {
			k, v, _ := l.evictBack()
			l.listMu.Unlock()
			l.onEviction(k, v)
			l.listMu.Lock()
		} else {
			l.evictBack()
		}
	}

	// Create new node
	idx := l.acquireNode()
	if idx < 0 {
		return
	}
	node := &l.nodePool[idx]
	node.key = key
	node.value = value
	node.expiration = exp
	l.m.Store(key, idx)
	l.addToFront(idx)
	l.size.Add(1)
}

func (l *LRU[K, V]) Get(key K) (V, bool) {
	idx, ok := l.m.Load(key)
	if !ok {
		var zero V
		return zero, false
	}

	l.listMu.Lock()
	defer l.listMu.Unlock()

	if idx < 0 || idx >= int64(len(l.nodePool)) {
		var zero V
		return zero, false
	}

	node := &l.nodePool[idx]
	if node.key != key {
		var zero V
		return zero, false
	}

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

func (l *LRU[K, V]) Peek(key K) (V, bool) {
	idx, ok := l.m.Load(key)
	if !ok {
		var zero V
		return zero, false
	}

	l.listMu.Lock()
	defer l.listMu.Unlock()

	if idx < 0 || idx >= int64(len(l.nodePool)) {
		var zero V
		return zero, false
	}

	node := &l.nodePool[idx]
	if node.key != key {
		var zero V
		return zero, false
	}

	if node.expiration > 0 && time.Now().UnixNano() > node.expiration {
		var zero V
		return zero, false
	}

	return node.value, true
}

func (l *LRU[K, V]) Delete(key K) bool {
	idx, ok := l.m.Load(key)
	if !ok {
		return false
	}

	l.listMu.Lock()
	if idx < 0 || idx >= int64(len(l.nodePool)) || l.nodePool[idx].key != key {
		l.listMu.Unlock()
		return false
	}
	l.m.Delete(key)
	l.removeFromList(idx)
	l.releaseNode(idx)
	l.listMu.Unlock()
	l.size.Add(-1)
	return true
}

func (l *LRU[K, V]) Has(key K) bool {
	_, ok := l.Peek(key)
	return ok
}

func (l *LRU[K, V]) Len() int {
	return int(l.size.Load())
}

func (l *LRU[K, V]) Clear() {
	l.listMu.Lock()
	defer l.listMu.Unlock()

	if l.onEviction != nil {
		l.m.Range(func(key K, idx int64) bool {
			if idx >= 0 && idx < int64(len(l.nodePool)) {
				node := &l.nodePool[idx]
				l.onEviction(node.key, node.value)
			}
			return true
		})
	}

	l.m.Clear()
	l.nodePool = l.nodePool[:0]
	l.head, l.tail, l.freeList = -1, -1, -1
	l.size.Store(0)
}

func (l *LRU[K, V]) Keys() []K {
	l.listMu.Lock()
	defer l.listMu.Unlock()

	keys := make([]K, 0, l.Len())
	now := time.Now().UnixNano()
	for idx := l.head; idx >= 0; {
		if idx >= int64(len(l.nodePool)) {
			break
		}
		node := &l.nodePool[idx]
		if node.expiration == 0 || node.expiration > now {
			keys = append(keys, node.key)
		}
		idx = node.next
	}
	return keys
}

func (l *LRU[K, V]) Values() []V {
	l.listMu.Lock()
	defer l.listMu.Unlock()

	values := make([]V, 0, l.Len())
	now := time.Now().UnixNano()
	for idx := l.head; idx >= 0; {
		if idx >= int64(len(l.nodePool)) {
			break
		}
		node := &l.nodePool[idx]
		if node.expiration == 0 || node.expiration > now {
			values = append(values, node.value)
		}
		idx = node.next
	}
	return values
}

func (l *LRU[K, V]) ForEach(fn func(K, V) bool) {
	l.listMu.Lock()
	defer l.listMu.Unlock()

	now := time.Now().UnixNano()
	for idx := l.head; idx >= 0; {
		if idx >= int64(len(l.nodePool)) {
			break
		}
		node := &l.nodePool[idx]
		nextIdx := node.next
		if node.expiration == 0 || node.expiration > now {
			if !fn(node.key, node.value) {
				return
			}
		}
		idx = nextIdx
	}
}

func (l *LRU[K, V]) GetOrSet(key K, value V, ttl time.Duration) (V, bool) {
	if v, ok := l.Get(key); ok {
		return v, true
	}

	var exp int64
	if ttl > 0 {
		exp = time.Now().Add(ttl).UnixNano()
	}

	l.listMu.Lock()
	defer l.listMu.Unlock()

	if idx, ok := l.m.Load(key); ok {
		if idx >= 0 && idx < int64(len(l.nodePool)) {
			node := &l.nodePool[idx]
			if node.key == key && (node.expiration == 0 || time.Now().UnixNano() <= node.expiration) {
				l.moveToFront(idx)
				return node.value, true
			}
			l.removeFromList(idx)
			l.releaseNode(idx)
			l.size.Add(-1)
		}
	}

	for int(l.size.Load()) >= l.maxSize {
		if l.onEviction != nil {
			k, v, _ := l.evictBack()
			l.listMu.Unlock()
			l.onEviction(k, v)
			l.listMu.Lock()
		} else {
			l.evictBack()
		}
	}

	idx := l.acquireNode()
	if idx < 0 {
		var zero V
		return zero, false
	}
	node := &l.nodePool[idx]
	node.key = key
	node.value = value
	node.expiration = exp
	l.addToFront(idx)
	l.m.Store(key, idx)
	l.size.Add(1)
	return value, false
}

func (l *LRU[K, V]) GetOrCompute(key K, fn func() (V, time.Duration)) V {
	if v, ok := l.Get(key); ok {
		return v
	}
	val, ttl := fn()
	actual, _ := l.GetOrSet(key, val, ttl)
	return actual
}

func (l *LRU[K, V]) Resize(maxSize int) {
	if maxSize <= 0 {
		maxSize = 1000
	}
	l.listMu.Lock()
	defer l.listMu.Unlock()
	l.maxSize = maxSize
	for int(l.size.Load()) > maxSize {
		if l.onEviction != nil {
			k, v, _ := l.evictBack()
			l.listMu.Unlock()
			l.onEviction(k, v)
			l.listMu.Lock()
		} else {
			l.evictBack()
		}
	}
}

func (l *LRU[K, V]) PurgeExpired() int {
	l.listMu.Lock()
	defer l.listMu.Unlock()

	now := time.Now().UnixNano()
	removed := 0
	for idx := l.head; idx >= 0; {
		if idx >= int64(len(l.nodePool)) {
			break
		}
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
