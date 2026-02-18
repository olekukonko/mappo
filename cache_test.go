package mappo

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCache_LoadStore(t *testing.T) {
	c := NewCache(CacheOptions{MaximumSize: 10})
	it := &Item{Value: "value"}
	c.Store("key", it)
	loadedIt, ok := c.Load("key")
	if !ok {
		t.Error("expected to load item")
	}
	if loadedIt.Value != "value" {
		t.Error("expected value to be 'value'")
	}
}

func TestCache_StoreTTL_Expiration(t *testing.T) {
	c := NewCache(CacheOptions{
		MaximumSize: 10,
		Now:         func() time.Time { return time.Now() },
	})
	it := &Item{Value: "value"}
	c.StoreTTL("key", it, time.Millisecond)
	time.Sleep(2 * time.Millisecond)
	_, ok := c.Load("key")
	if ok {
		t.Error("expected item to be expired")
	}
}

func TestCache_LoadOrStore(t *testing.T) {
	c := NewCache(CacheOptions{MaximumSize: 10})
	it := &Item{Value: "value"}
	loadedIt, loaded := c.LoadOrStore("key", it)
	if loaded {
		t.Error("expected not loaded on first call")
	}
	if loadedIt.Value != "value" {
		t.Error("expected value 'value'")
	}
	loadedIt, loaded = c.LoadOrStore("key", &Item{Value: "new"})
	if !loaded {
		t.Error("expected loaded on second call")
	}
	if loadedIt.Value != "value" {
		t.Error("expected original value")
	}
}

func TestCache_LoadAndDelete(t *testing.T) {
	c := NewCache(CacheOptions{MaximumSize: 10})
	it := &Item{Value: "value"}
	c.Store("key", it)
	loadedIt, ok := c.LoadAndDelete("key")
	if !ok {
		t.Error("expected to load and delete")
	}
	if loadedIt.Value != "value" {
		t.Error("expected value 'value'")
	}
	_, ok = c.Load("key")
	if ok {
		t.Error("expected deleted")
	}
}

func TestCache_GetValue(t *testing.T) {
	c := NewCache(CacheOptions{MaximumSize: 10})
	c.Store("key", &Item{Value: "value"})
	val, ok := c.GetValue("key")
	if !ok {
		t.Error("expected to get value")
	}
	if val != "value" {
		t.Error("expected 'value'")
	}
}

func TestCache_GetOrSet(t *testing.T) {
	c := NewCache(CacheOptions{MaximumSize: 10})
	val, loaded := c.GetOrSet("key", "value", 0)
	if loaded {
		t.Error("expected not loaded")
	}
	if val != "value" {
		t.Error("expected 'value'")
	}
	val, loaded = c.GetOrSet("key", "new", 0)
	if !loaded {
		t.Error("expected loaded")
	}
	if val != "value" {
		t.Error("expected original 'value'")
	}
}

func TestCache_GetOrCompute(t *testing.T) {
	c := NewCache(CacheOptions{MaximumSize: 10})
	count := 0
	val := c.GetOrCompute("key", func() (any, time.Duration) {
		count++
		return "value", 0
	})
	if val != "value" {
		t.Error("expected 'value'")
	}
	if count != 1 {
		t.Error("expected count 1")
	}
	val = c.GetOrCompute("key", func() (any, time.Duration) {
		count++
		return "new", 0
	})
	if val != "value" {
		t.Error("expected original 'value'")
	}
	if count != 1 {
		t.Error("expected count still 1")
	}
}

func TestCache_Delete(t *testing.T) {
	c := NewCache(CacheOptions{MaximumSize: 10})
	c.Store("key", &Item{Value: "value"})
	c.Delete("key")
	_, ok := c.Load("key")
	if ok {
		t.Error("expected deleted")
	}
}

func TestCache_Has(t *testing.T) {
	c := NewCache(CacheOptions{MaximumSize: 10})
	c.Store("key", &Item{Value: "value"})
	if !c.Has("key") {
		t.Error("expected has key")
	}
	c.Delete("key")
	if c.Has("key") {
		t.Error("expected not has key")
	}
}

func TestCache_LenClear(t *testing.T) {
	c := NewCache(CacheOptions{MaximumSize: 10})
	c.Store("key1", &Item{Value: "1"})
	c.Store("key2", &Item{Value: "2"})
	if c.Len() != 2 {
		t.Error("expected len 2")
	}
	c.Clear()
	if c.Len() != 0 {
		t.Error("expected len 0 after clear")
	}
}

func TestCache_RangeKeys(t *testing.T) {
	c := NewCache(CacheOptions{MaximumSize: 10})
	c.Store("key1", &Item{Value: "1"})
	c.Store("key2", &Item{Value: "2"})
	keys := c.Keys()
	if len(keys) != 2 {
		t.Error("expected 2 keys")
	}
	count := 0
	c.Range(func(key string, item *Item) bool {
		count++
		return true
	})
	if count != 2 {
		t.Error("expected range count 2")
	}
}

func TestCache_Stats(t *testing.T) {
	c := NewCache(CacheOptions{MaximumSize: 10})
	_, _ = c.Load("miss")
	c.Store("key", &Item{Value: "value"})
	_, _ = c.Load("key")
	stats := c.Stats()
	if stats.Hits != 1 {
		t.Error("expected hits 1")
	}
	if stats.Misses != 1 {
		t.Error("expected misses 1")
	}
	if stats.Size != 1 {
		t.Error("expected size 1")
	}
	if stats.Capacity != 10 {
		t.Error("expected capacity 10")
	}
}

func TestCache_OnDelete(t *testing.T) {
	var deleted int32
	c := NewCache(CacheOptions{
		MaximumSize: 1,
		OnDelete: func(key string, it *Item) {
			atomic.StoreInt32(&deleted, 1)
		},
	})
	c.Store("key1", &Item{Value: "1"})
	c.Store("key2", &Item{Value: "2"})
	time.Sleep(100 * time.Millisecond)
	if atomic.LoadInt32(&deleted) == 0 {
		t.Error("expected OnDelete called")
	}
}

func TestCache_ConcurrentAccess(t *testing.T) {
	c := NewCache(CacheOptions{MaximumSize: 1000})
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", i)
			c.Store(key, &Item{Value: i})
			loadedIt, ok := c.Load(key)
			if !ok {
				t.Error("expected load")
			}
			if loadedIt.Value.(int) != i {
				t.Error("expected value match")
			}
		}(i)
	}
	wg.Wait()
	if c.Len() != 100 {
		t.Error("expected len 100")
	}
}

func BenchmarkCache_Set(b *testing.B) {
	c := NewCache(CacheOptions{MaximumSize: b.N})
	it := &Item{Value: "value"}
	for i := 0; i < b.N; i++ {
		c.Store(fmt.Sprintf("key%d", i), it)
	}
}

func BenchmarkCache_Load(b *testing.B) {
	c := NewCache(CacheOptions{MaximumSize: b.N})
	for i := 0; i < b.N; i++ {
		c.Store(fmt.Sprintf("key%d", i), &Item{Value: "value"})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Load(fmt.Sprintf("key%d", i))
	}
}
