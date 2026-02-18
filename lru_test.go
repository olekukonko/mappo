package mappo

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestLRU_BasicOperations(t *testing.T) {
	l := NewLRU[string, string](2)
	l.Set("key1", "value1")
	l.Set("key2", "value2")
	val, ok := l.Get("key1")
	if !ok {
		t.Error("expected to get key1")
	}
	if val != "value1" {
		t.Error("expected value1")
	}
	l.Set("key3", "value3")
	_, ok = l.Get("key2")
	if ok {
		t.Error("expected key2 evicted")
	}
}

func TestLRU_TTL(t *testing.T) {
	l := NewLRU[string, string](10)
	l.SetWithTTL("key", "value", time.Millisecond)
	time.Sleep(2 * time.Millisecond)
	_, ok := l.Get("key")
	if ok {
		t.Error("expected expired")
	}
}

func TestLRU_Peek(t *testing.T) {
	l := NewLRU[string, string](2)
	l.Set("key1", "value1")
	l.Set("key2", "value2")
	val, ok := l.Peek("key1")
	if !ok {
		t.Error("expected peek key1")
	}
	if val != "value1" {
		t.Error("expected value1")
	}
	l.Set("key3", "value3")
	_, ok = l.Get("key1")
	if ok {
		t.Error("expected key1 evicted")
	}
	val, ok = l.Get("key2")
	if !ok {
		t.Error("expected key2 not evicted")
	}
	if val != "value2" {
		t.Error("expected value2")
	}
}

func TestLRU_PurgeExpired(t *testing.T) {
	l := NewLRU[string, string](10)
	l.SetWithTTL("key1", "value1", time.Millisecond)
	l.Set("key2", "value2")
	time.Sleep(2 * time.Millisecond)
	removed := l.PurgeExpired()
	if removed != 1 {
		t.Error("expected 1 removed")
	}
	if l.Len() != 1 {
		t.Error("expected len 1")
	}
}

func TestLRU_OnEviction(t *testing.T) {
	evicted := false
	l := NewLRUWithConfig[string, string](LRUConfig[string, string]{
		MaxSize:    1,
		OnEviction: func(key string, value string) { evicted = true },
	})
	l.Set("key1", "value1")
	l.Set("key2", "value2")
	if !evicted {
		t.Error("expected OnEviction called")
	}
}

func TestLRU_KeysForEach(t *testing.T) {
	l := NewLRU[string, string](10)
	l.Set("key1", "value1")
	l.Set("key2", "value2")
	keys := l.Keys()
	if len(keys) != 2 {
		t.Error("expected 2 keys")
	}
	count := 0
	l.ForEach(func(k string, v string) bool {
		count++
		return true
	})
	if count != 2 {
		t.Error("expected for each 2")
	}
}

func TestLRU_Concurrent(t *testing.T) {
	l := NewLRUWithConfig[string, int](LRUConfig[string, int]{MaxSize: 100})
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", i)
			l.Set(key, i)
			val, ok := l.Get(key)
			if !ok {
				t.Error("expected get")
			}
			if val != i {
				t.Error("expected value match")
			}
		}(i)
	}
	wg.Wait()
	if l.Len() != 100 {
		t.Error("expected len 100")
	}
}

func BenchmarkLRU_Set(b *testing.B) {
	l := NewLRU[string, string](b.N)
	for i := 0; i < b.N; i++ {
		l.Set(fmt.Sprintf("key%d", i), "value")
	}
}

func BenchmarkLRU_Get(b *testing.B) {
	l := NewLRU[string, string](b.N)
	for i := 0; i < b.N; i++ {
		l.Set(fmt.Sprintf("key%d", i), "value")
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Get(fmt.Sprintf("key%d", i))
	}
}
