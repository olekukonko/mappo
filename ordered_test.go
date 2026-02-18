package mappo

import (
	"fmt"
	"sync"
	"testing"
)

func TestOrdered_Basic(t *testing.T) {
	o := NewOrdered[string, int]()
	o.Set("key1", 1)
	o.Set("key2", 2)
	keys := o.Keys()
	if len(keys) != 2 || keys[0] != "key1" || keys[1] != "key2" {
		t.Error("expected ordered keys")
	}
	val, ok := o.Get("key1")
	if !ok {
		t.Error("expected get")
	}
	if val != 1 {
		t.Error("expected 1")
	}
}

func TestOrdered_SetFront(t *testing.T) {
	o := NewOrdered[string, int]()
	o.Set("key1", 1)
	o.SetFront("key2", 2)
	keys := o.Keys()
	if len(keys) != 2 || keys[0] != "key2" || keys[1] != "key1" {
		t.Error("expected front order")
	}
}

func TestOrdered_DeleteAt(t *testing.T) {
	o := NewOrdered[string, int]()
	o.Set("key1", 1)
	o.Set("key2", 2)
	ok := o.DeleteAt(0)
	if !ok {
		t.Error("expected delete at 0")
	}
	keys := o.Keys()
	if len(keys) != 1 || keys[0] != "key2" {
		t.Error("expected remaining key2")
	}
}

func TestOrdered_GetAt(t *testing.T) {
	o := NewOrdered[string, int]()
	o.Set("key1", 1)
	o.Set("key2", 2)
	k, v, ok := o.GetAt(1)
	if !ok {
		t.Error("expected get at 1")
	}
	if k != "key2" || v != 2 {
		t.Error("expected key2, 2")
	}
}

func TestOrdered_FrontBack(t *testing.T) {
	o := NewOrdered[string, int]()
	o.Set("key1", 1)
	o.Set("key2", 2)
	fk, fv, ok := o.Front()
	if !ok || fk != "key1" || fv != 1 {
		t.Error("expected front key1,1")
	}
	bk, bv, ok := o.Back()
	if !ok || bk != "key2" || bv != 2 {
		t.Error("expected back key2,2")
	}
}

func TestOrdered_ForEach(t *testing.T) {
	o := NewOrdered[string, int]()
	o.Set("key1", 1)
	o.Set("key2", 2)
	count := 0
	o.ForEach(func(k string, v int) bool {
		count++
		return true
	})
	if count != 2 {
		t.Error("expected 2")
	}
}

func TestOrdered_Concurrent(t *testing.T) {
	o := NewOrderedWithConfig[string, int](OrderedConfig{Concurrent: true})
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key%d", i)
			o.Set(key, i)
			val, ok := o.Get(key)
			if !ok {
				t.Error("expected get")
			}
			if val != i {
				t.Error("expected value match")
			}
		}(i)
	}
	wg.Wait()
	if o.Len() != 100 {
		t.Error("expected len 100")
	}
}

func BenchmarkOrdered_Set(b *testing.B) {
	o := NewOrdered[int, int]()
	for i := 0; i < b.N; i++ {
		o.Set(i, i)
	}
}

func BenchmarkOrdered_Get(b *testing.B) {
	o := NewOrdered[int, int]()
	for i := 0; i < b.N; i++ {
		o.Set(i, i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		o.Get(i)
	}
}
