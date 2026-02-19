package mappo

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// ==================== TESTS ====================

func TestConcurrent_Basic(t *testing.T) {
	c := NewConcurrent[string, int]()

	c.Set("key1", 100)
	val, ok := c.Get("key1")
	if !ok || val != 100 {
		t.Errorf("Expected 100, got %d, ok=%v", val, ok)
	}

	_, ok = c.Get("nonexistent")
	if ok {
		t.Error("Expected false for non-existent key")
	}
}

func TestConcurrent_TTL(t *testing.T) {
	c := NewConcurrent[string, int]()

	c.SetTTL("key1", 100, 50*time.Millisecond)

	val, ok := c.Get("key1")
	if !ok || val != 100 {
		t.Error("Key should exist immediately after SetTTL")
	}

	time.Sleep(100 * time.Millisecond)

	_, ok = c.Get("key1")
	if ok {
		t.Error("Key should be expired")
	}
}

func TestConcurrent_SetIfAbsent(t *testing.T) {
	c := NewConcurrent[string, int]()

	val, loaded := c.SetIfAbsent("key1", 100)
	if loaded {
		t.Error("First SetIfAbsent should not load existing")
	}

	val, loaded = c.SetIfAbsent("key1", 200)
	if !loaded || val != 100 {
		t.Errorf("Second SetIfAbsent should load existing 100, got %d, loaded=%v", val, loaded)
	}
}

func TestConcurrent_Compute(t *testing.T) {
	c := NewConcurrent[string, int]()

	result := c.Compute("counter", func(current int, exists bool) (int, bool) {
		if !exists {
			return 1, true
		}
		return current + 1, true
	})
	if result != 1 {
		t.Errorf("Expected 1, got %d", result)
	}

	result = c.Compute("counter", func(current int, exists bool) (int, bool) {
		return current + 1, true
	})
	if result != 2 {
		t.Errorf("Expected 2, got %d", result)
	}

	c.Compute("counter", func(current int, exists bool) (int, bool) {
		return 0, false // delete
	})

	_, ok := c.Get("counter")
	if ok {
		t.Error("Key should be deleted")
	}
}

func TestConcurrent_ConcurrentAccess(t *testing.T) {
	c := NewConcurrent[int, int]()
	const numGoroutines = 100
	const numOps = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := (id*numOps + j) % 100
				c.Set(key, j)
				c.Get(key)
			}
		}(i)
	}

	wg.Wait()
}

func TestConcurrent_Clear(t *testing.T) {
	c := NewConcurrent[string, int]()

	c.Set("a", 1)
	c.Set("b", 2)
	c.Clear()

	_, ok := c.Get("a")
	if ok {
		t.Error("Map should be empty after Clear")
	}
}

func TestConcurrent_Len(t *testing.T) {
	c := NewConcurrent[string, int]()

	if c.Len() != 0 {
		t.Errorf("Expected empty map, got len %d", c.Len())
	}

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	if c.Len() < 3 {
		t.Errorf("Expected at least 3 elements, got %d", c.Len())
	}
}

func TestConcurrent_Range(t *testing.T) {
	c := NewConcurrent[string, int]()

	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3)

	count := 0
	sum := 0
	c.Range(func(k string, v int) bool {
		count++
		sum += v
		return true
	})

	if count != 3 {
		t.Errorf("Expected 3 items, got %d", count)
	}
	if sum != 6 {
		t.Errorf("Expected sum 6, got %d", sum)
	}
}

func TestConcurrent_KeysValues(t *testing.T) {
	c := NewConcurrent[string, int]()

	c.Set("a", 1)
	c.Set("b", 2)

	keys := c.Keys()
	if len(keys) != 2 {
		t.Errorf("Expected 2 keys, got %d", len(keys))
	}

	values := c.Values()
	if len(values) != 2 {
		t.Errorf("Expected 2 values, got %d", len(values))
	}
}

// Test API compatibility with Sharded
func TestConcurrent_APIMatch(t *testing.T) {
	c := NewConcurrent[string, int]()

	c.Set("key", 1)

	val, ok := c.Get("key")
	if !ok || val != 1 {
		t.Error("Get failed")
	}

	actual, loaded := c.SetIfAbsent("key", 2)
	if !loaded || actual != 1 {
		t.Error("SetIfAbsent failed")
	}

	c.Update("key", func(curr int, exists bool) int {
		return curr + 10
	})

	val, _ = c.Get("key")
	if val != 11 {
		t.Errorf("Update failed, expected 11, got %d", val)
	}

	c.Compute("key", func(curr int, exists bool) (int, bool) {
		return curr * 2, true
	})

	val, _ = c.Get("key")
	if val != 22 {
		t.Errorf("Compute failed, expected 22, got %d", val)
	}

	if !c.Has("key") {
		t.Error("Has failed")
	}

	c.Delete("key")
	if c.Has("key") {
		t.Error("Delete failed")
	}

	c.Set("x", 1)
	if c.Size() != c.Len() {
		t.Error("Size should equal Len")
	}
}

// ==================== BENCHMARKS ====================

func BenchmarkConcurrent_Set(b *testing.B) {
	c := NewConcurrent[string, int]()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Set(fmt.Sprintf("key%d", i), i)
			i++
		}
	})
}

func BenchmarkConcurrent_Get(b *testing.B) {
	c := NewConcurrent[string, int]()
	for i := 0; i < 1000; i++ {
		c.Set(fmt.Sprintf("key%d", i), i)
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Get(fmt.Sprintf("key%d", i%1000))
			i++
		}
	})
}

func BenchmarkConcurrent_SetGet(b *testing.B) {
	c := NewConcurrent[string, int]()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key%d", i%100)
			c.Set(key, i)
			c.Get(key)
			i++
		}
	})
}

func BenchmarkConcurrent_SetIfAbsent(b *testing.B) {
	c := NewConcurrent[string, int]()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.SetIfAbsent(fmt.Sprintf("key%d", i), i)
			i++
		}
	})
}

func BenchmarkConcurrent_Compute(b *testing.B) {
	c := NewConcurrent[string, int]()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Compute(fmt.Sprintf("key%d", i%100), func(curr int, exists bool) (int, bool) {
				return curr + 1, true
			})
			i++
		}
	})
}

func BenchmarkConcurrent_Update(b *testing.B) {
	c := NewConcurrent[string, int]()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Update(fmt.Sprintf("key%d", i%100), func(curr int, exists bool) int {
				return curr + 1
			})
			i++
		}
	})
}

type rateLimitEntry struct {
	lastSeen int64
	count    int32
}

func BenchmarkSharded_RateLimitPattern(b *testing.B) {
	s := NewShardedWithConfig[string, *rateLimitEntry](ShardedConfig{ShardCount: 16})
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("client%d", i%1000)
			s.Compute(key, func(curr *rateLimitEntry, exists bool) (*rateLimitEntry, bool) {
				if exists {
					curr.count++
					curr.lastSeen = time.Now().Unix()
					return curr, true
				}
				return &rateLimitEntry{count: 1, lastSeen: time.Now().Unix()}, true
			})
			i++
		}
	})
}

func BenchmarkConcurrent_RateLimitPattern(b *testing.B) {
	c := NewConcurrent[string, *rateLimitEntry]()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("client%d", i%1000)
			c.Compute(key, func(curr *rateLimitEntry, exists bool) (*rateLimitEntry, bool) {
				if exists {
					curr.count++
					curr.lastSeen = time.Now().Unix()
					return curr, true
				}
				return &rateLimitEntry{count: 1, lastSeen: time.Now().Unix()}, true
			})
			i++
		}
	})
}

func BenchmarkConcurrent_Set_Memory(b *testing.B) {
	c := NewConcurrent[string, int]()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key%d", i), i)
	}
}

func BenchmarkSharded_Set_Memory(b *testing.B) {
	s := NewShardedWithConfig[string, int](ShardedConfig{ShardCount: 16})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Set(fmt.Sprintf("key%d", i), i)
	}
}

func BenchmarkSharded_HighContention(b *testing.B) {
	s := NewShardedWithConfig[string, int](ShardedConfig{ShardCount: 16})
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			s.Compute("samekey", func(curr int, exists bool) (int, bool) {
				return curr + 1, true
			})
		}
	})
}

func BenchmarkConcurrent_HighContention(b *testing.B) {
	c := NewConcurrent[string, int]()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Compute("samekey", func(curr int, exists bool) (int, bool) {
				return curr + 1, true
			})
		}
	})
}
