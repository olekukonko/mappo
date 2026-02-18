// sharded_test.go
package mappo

import (
	"fmt"
	"sync"
	"testing"
)

func TestSharded_Basic(t *testing.T) {
	s := NewSharded[string, int]()
	s.Set("key", 42)
	val, ok := s.Get("key")
	if !ok {
		t.Error("expected get")
	}
	if val != 42 {
		t.Error("expected 42")
	}
	s.Delete("key")
	_, ok = s.Get("key")
	if ok {
		t.Error("expected deleted")
	}
}

func TestSharded_Compute(t *testing.T) {
	s := NewSharded[string, int]()
	newVal := s.Compute("key", func(curr int, exists bool) (int, bool) {
		if exists {
			t.Error("expected not exists")
		}
		return 42, true
	})
	if newVal != 42 {
		t.Error("expected 42")
	}
	newVal = s.Compute("key", func(curr int, exists bool) (int, bool) {
		if !exists {
			t.Error("expected exists")
		}
		if curr != 42 {
			t.Error("expected curr 42")
		}
		return 0, false // delete
	})
	_, ok := s.Get("key")
	if ok {
		t.Error("expected deleted")
	}
}

func TestSharded_ClearIf(t *testing.T) {
	s := NewSharded[string, int]()
	s.Set("key1", 1)
	s.Set("key2", 2)
	removed := s.ClearIf(func(k string, v int) bool { return v == 1 })
	if removed != 1 {
		t.Error("expected 1 removed")
	}
	if s.Len() != 1 {
		t.Error("expected len 1")
	}
}

func TestSharded_KeysValues(t *testing.T) {
	s := NewSharded[string, int]()
	s.Set("key1", 1)
	s.Set("key2", 2)
	keys := s.Keys()
	if len(keys) != 2 {
		t.Error("expected 2 keys")
	}
	values := s.Values()
	if len(values) != 2 {
		t.Error("expected 2 values")
	}
}

func TestSharded_HasGetOrSet(t *testing.T) {
	s := NewSharded[string, int]()
	actual, loaded := s.GetOrSet("key", 42)
	if loaded {
		t.Error("expected not loaded")
	}
	if actual != 42 {
		t.Error("expected 42")
	}
	if !s.Has("key") {
		t.Error("expected has key")
	}
	actual, loaded = s.GetOrSet("key", 100)
	if !loaded {
		t.Error("expected loaded")
	}
	if actual != 42 {
		t.Error("expected original 42")
	}
}

func TestSharded_ForEach(t *testing.T) {
	s := NewSharded[string, int]()
	s.Set("key1", 1)
	s.Set("key2", 2)
	count := 0
	s.ForEach(func(k string, v int) bool {
		count++
		return true
	})
	if count != 2 {
		t.Error("expected 2")
	}
}

func TestSharded_Concurrent(t *testing.T) {
	s := NewSharded[string, int]()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", i)
			s.Set(key, i)
			val, ok := s.Get(key)
			if !ok {
				t.Error("expected get")
			}
			if val != i {
				t.Error("expected value match")
			}
		}(i)
	}
	wg.Wait()
	if s.Len() != 100 {
		t.Error("expected len 100")
	}
}

func BenchmarkSharded_Set(b *testing.B) {
	s := NewSharded[string, int]()
	for i := 0; i < b.N; i++ {
		s.Set(fmt.Sprintf("key%d", i), i)
	}
}

func BenchmarkSharded_Get(b *testing.B) {
	s := NewSharded[string, int]()
	for i := 0; i < b.N; i++ {
		s.Set(fmt.Sprintf("key%d", i), i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Get(fmt.Sprintf("key%d", i))
	}
}
