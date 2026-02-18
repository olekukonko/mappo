package mappo

import "testing"

func TestSet_Basic(t *testing.T) {
	s := NewSet[int](1, 2, 3)
	if !s.Has(2) {
		t.Error("expected has 2")
	}
	s.Remove(2)
	if s.Has(2) {
		t.Error("expected not has 2")
	}
	if s.Len() != 2 {
		t.Error("expected len 2")
	}
}

func TestSet_Union(t *testing.T) {
	s1 := NewSet[int](1, 2)
	s2 := NewSet[int](2, 3)
	union := s1.Union(s2)
	if union.Len() != 3 {
		t.Error("expected len 3")
	}
	if !union.Has(1) || !union.Has(2) || !union.Has(3) {
		t.Error("expected all elements")
	}
}

func TestSet_Intersection(t *testing.T) {
	s1 := NewSet[int](1, 2, 3)
	s2 := NewSet[int](2, 3, 4)
	inter := s1.Intersection(s2)
	if inter.Len() != 2 {
		t.Error("expected len 2")
	}
	if !inter.Has(2) || !inter.Has(3) {
		t.Error("expected 2 and 3")
	}
}

func TestSet_Difference(t *testing.T) {
	s1 := NewSet[int](1, 2, 3)
	s2 := NewSet[int](2, 4)
	diff := s1.Difference(s2)
	if diff.Len() != 2 {
		t.Error("expected len 2")
	}
	if !diff.Has(1) || !diff.Has(3) {
		t.Error("expected 1 and 3")
	}
}

func TestSet_ForEach(t *testing.T) {
	s := NewSet[int](1, 2)
	count := 0
	s.ForEach(func(v int) {
		count++
	})
	if count != 2 {
		t.Error("expected 2")
	}
}

func TestSet_Clear(t *testing.T) {
	s := NewSet[int](1, 2)
	s.Clear()
	if s.Len() != 0 {
		t.Error("expected len 0")
	}
}

func BenchmarkSet_Add(b *testing.B) {
	s := NewSet[int]()
	for i := 0; i < b.N; i++ {
		s.Add(i)
	}
}

func BenchmarkSet_Has(b *testing.B) {
	s := NewSet[int]()
	for i := 0; i < b.N; i++ {
		s.Add(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Has(i)
	}
}
