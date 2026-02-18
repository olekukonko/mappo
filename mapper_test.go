package mappo

import "testing"

func TestMapper_Basic(t *testing.T) {
	m := NewMapper[string, int]()
	m.Set("key", 42)
	val := m.Get("key")
	if val != 42 {
		t.Error("expected 42")
	}
	if !m.Has("key") {
		t.Error("expected has key")
	}
	m.Delete("key")
	if m.Has("key") {
		t.Error("expected not has key")
	}
}

func TestMapper_Nil(t *testing.T) {
	var m Mapper[string, int]
	if m.Get("key") != 0 {
		t.Error("expected zero")
	}
	if m.Has("key") {
		t.Error("expected not has")
	}
	if m.Len() != 0 {
		t.Error("expected len 0")
	}
}

func TestMapper_Filter(t *testing.T) {
	m := NewMapper[int, string]()
	m.Set(1, "one")
	m.Set(2, "two")
	filtered := m.Filter(func(k int, v string) bool { return k%2 == 0 })
	if filtered.Len() != 1 {
		t.Error("expected len 1")
	}
	if filtered.Get(2) != "two" {
		t.Error("expected 'two'")
	}
}

func TestMapper_MapValues(t *testing.T) {
	m := NewMapper[string, int]()
	m.Set("key", 1)
	mapped := m.MapValues(func(v int) int { return v + 1 })
	if mapped.Get("key") != 2 {
		t.Error("expected 2")
	}
}

func TestMapper_Clone(t *testing.T) {
	m := NewMapper[string, int]()
	m.Set("key", 42)
	clone := m.Clone()
	if clone.Get("key") != 42 {
		t.Error("expected 42")
	}
}

func TestMapper_ToSlice(t *testing.T) {
	m := NewMapper[string, int]()
	m.Set("key", 42)
	slice := m.ToSlice()
	if len(slice) != 1 {
		t.Error("expected len 1")
	}
	if slice[0].Key != "key" || slice[0].Value != 42 {
		t.Error("expected key-value pair")
	}
}

func TestMapper_SortedKeys(t *testing.T) {
	m := NewMapper[int, string]()
	m.Set(3, "three")
	m.Set(1, "one")
	m.Set(2, "two")
	keys := m.SortedKeys()
	if len(keys) != 3 || keys[0] != 1 || keys[1] != 2 || keys[2] != 3 {
		t.Error("expected sorted [1 2 3]")
	}
}

func TestMapper_ForEach(t *testing.T) {
	m := NewMapper[string, int]()
	m.Set("key1", 1)
	m.Set("key2", 2)
	count := 0
	m.ForEach(func(k string, v int) {
		count++
	})
	if count != 2 {
		t.Error("expected 2")
	}
}

func TestNewBoolMapper(t *testing.T) {
	m := NewBoolMapper[string]("a", "b")
	if !m.Get("a") {
		t.Error("expected true")
	}
	if m.Get("c") {
		t.Error("expected false")
	}
}

func TestNewIntMapper(t *testing.T) {
	m := NewIntMapper[string]("a", "b")
	if m.Get("a") != 0 {
		t.Error("expected 0")
	}
}

func TestNewIdentityMapper(t *testing.T) {
	m := NewIdentityMapper[string]("a", "b")
	if m.Get("a") != "a" {
		t.Error("expected 'a'")
	}
}

func TestMerge(t *testing.T) {
	m1 := NewMapper[string, int]()
	m1.Set("key", 1)
	m2 := NewMapper[string, int]()
	m2.Set("key", 2)
	merged := Merge(m1, m2)
	if merged.Get("key") != 2 {
		t.Error("expected 2")
	}
}

func BenchmarkMapper_Set(b *testing.B) {
	m := NewMapper[int, int]()
	for i := 0; i < b.N; i++ {
		m.Set(i, i)
	}
}

func BenchmarkMapper_Get(b *testing.B) {
	m := NewMapper[int, int]()
	for i := 0; i < b.N; i++ {
		m.Set(i, i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Get(i)
	}
}
