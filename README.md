# mappo

Fast map implementations for Go with advanced features like TTL, LRU eviction, sharding, and thread-safe operations.

## Features

- **Cache** - High-performance concurrent map with TTL support using [Otter](https://github.com/maypok86/otter) as the backend
- **Concurrent** - Lock-free concurrent map using [xsync](https://github.com/puzpuzpuz/xsync) with optional TTL
- **Sharded** - Sharded map for high-concurrency scenarios, reducing lock contention
- **LRU** - Thread-safe LRU map with TTL and configurable eviction callbacks
- **Ordered** - Insertion-order-preserving map with O(1) operations
- **Mapper** - Enhanced built-in map with functional operations
- **Set** - Generic set implementation based on Mapper

## Installation

```bash
go get github.com/olekukonko/mappo
```

## Quick Start

### Cache

```go
import "github.com/olekukonko/mappo"

// Create a cache with max size
cache := mappo.NewCache(mappo.CacheOptions{
    MaximumSize: 10000,
    OnDelete: func(key string, item *mappo.Item) {
        // Cleanup logic
    },
})

// Store with TTL
item := &mappo.Item{Value: "data"}
cache.StoreTTL("key", item, 5*time.Minute)

// Load
if item, ok := cache.Load("key"); ok {
    value := item.Value.(string)
}

// GetOrCompute for lazy loading
data := cache.GetOrCompute("key", func() (any, time.Duration) {
    return expensiveOperation(), 10 * time.Minute
})
```

### Concurrent Map

```go
// Thread-safe map with TTL support
m := mappo.NewConcurrent[string, User]()

m.Set("user:1", user)
m.SetTTL("session:123", session, 30*time.Minute)

if user, ok := m.Get("user:1"); ok {
    // Use user
}

// Atomic compute
newVal := m.Compute("counter", func(current int, exists bool) (int, bool) {
    return current + 1, true // increment and keep
})
```

### Sharded Map

```go
// Sharded for high concurrency - reduces lock contention
sharded := mappo.NewSharded[string, Session]()

// Configure shard count
sharded := mappo.NewShardedWithConfig[string, Session](mappo.ShardedConfig{
    ShardCount: 64, // Rounds up to power of 2
})

sharded.Set("session:123", session)
val, ok := sharded.Get("session:123")

// Atomic operations
sharded.Update("counter", func(current int64, exists bool) int64 {
    return current + 1
})

// Compare-and-swap with fast path for comparable types
swapped := sharded.CompareAndSwap("key", oldVal, newVal)
```

### LRU Cache

```go
// LRU with TTL and eviction callback
lru := mappo.NewLRUWithConfig[string, Data](mappo.LRUConfig[string, Data]{
    MaxSize: 1000,
    TTL:     5 * time.Minute,
    OnEviction: func(key string, val Data) {
        // Cleanup (close files, etc.)
    },
})

lru.Set("key", data)
lru.SetWithTTL("temp", data, 30*time.Second) // Override default TTL

if data, ok := lru.Get("key"); ok {
    // Recently used, moved to front
}

// Peek without affecting LRU order
if data, ok := lru.Peek("key"); ok {
    // ...
}

// Resize at runtime
lru.Resize(500)
```

### Ordered Map

```go
// Maintains insertion order, safe for concurrent use
ordered := mappo.NewOrderedWithConfig[string, int](mappo.OrderedConfig{
    Concurrent: true, // Enable mutex protection
})

ordered.Set("a", 1)
ordered.Set("b", 2)
ordered.SetFront("c", 3) // Add to front

// Iterate in order
ordered.ForEach(func(key string, val int) bool {
    fmt.Println(key, val)
    return true // continue
})

// Move elements
ordered.MoveToFront("b")
ordered.MoveToBack("a")

// Get by index (O(n))
key, val, ok := ordered.GetAt(0)

// Pop from ends
key, val, ok = ordered.PopFront()
```

### Mapper (Enhanced Map)

```go
// Functional operations on regular maps
m := mappo.NewMapper[string, int]()
m.Set("a", 1).Set("b", 2)

// Safe operations
val := m.Get("a")          // 0 if missing
val, ok := m.OK("a")       // Check existence
val, ok = m.Pop("a")       // Get and delete

// Functional methods
evens := m.Filter(func(k string, v int) bool {
    return v%2 == 0
})

doubled := m.MapValues(func(v int) int {
    return v * 2
})

// Set operations
keys := m.Keys()
values := m.Values()
cloned := m.Clone()

// Merge
combined := mappo.Merge(m1, m2, m3)
```

### Set

```go
s := mappo.NewSet("a", "b", "c")
s.Add("d")
s.Remove("b")

if s.Has("a") {
    // ...
}

// Set operations
union := s1.Union(s2)
intersection := s1.Intersection(s2)
diff := s1.Difference(s2)

// Check relationships
if s1.IsSubset(s2) {
    // ...
}
```

## Performance

Benchmarks on Apple M3 Pro:

| Operation | ns/op | B/op | allocs/op |
|-----------|-------|------|-----------|
| Cache.Set | 357 | 132 | 3 |
| Cache.Load | 296 | 24 | 2 |
| Concurrent.Set | 43 | 68 | 4 |
| Concurrent.Get | 13 | 13 | 1 |
| Sharded.Set | 355 | 113 | 3 |
| Sharded.Get | 185 | 23 | 1 |
| LRU.Set | 322 | 146 | 3 |
| LRU.Get | 184 | 23 | 1 |
| Mapper.Set | 75 | 49 | 0 |
| Mapper.Get | 36 | 0 | 0 |

## Design Decisions

### Why not just use `sync.Map`?

- `sync.Map` is `any->any` and not type-safe
- No TTL support
- No LRU or ordering guarantees
- Poor performance for typed data (boxing/unboxing)

### Why multiple implementations?

Different use cases need different trade-offs:

| Type | Best For | Thread-Safe | TTL | Ordered |
|------|----------|-------------|-----|---------|
| Cache | High-performance caching | ✓ | ✓ | ✗ |
| Concurrent | General concurrent map | ✓ | ✓ | ✗ |
| Sharded | Extreme concurrency (10M+ ops/s) | ✓ | ✗ | ✗ |
| LRU | Memory-bound caching | ✓ | ✓ | ✗ |
| Ordered | Sequenced processing | Optional | ✗ | ✓ |
| Mapper | Functional operations | ✗ | ✗ | ✗ |

### API Compatibility

`Concurrent` and `Sharded` share compatible APIs where possible:

- `Get`, `Set`, `Delete`, `Has`, `Len`
- `Compute`, `Update`, `SetIfAbsent`, `GetOrSet`
- `ForEach`, `Keys`, `Values`
- `Clear`, `ClearIf`, `Replace`, `CompareAndSwap`

This allows easy swapping based on performance needs.

## Dependencies

- [github.com/maypok86/otter](https://github.com/maypok86/otter) - High-performance cache backend
- [github.com/puzpuzpuz/xsync](https://github.com/puzpuzpuz/xsync) - Lock-free data structures

## License

MIT License - see LICENSE file