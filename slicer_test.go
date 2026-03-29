package mappo

import (
	"sync"
	"testing"
)

// Test basic append and get functionality.
func TestSlicerBasic(t *testing.T) {
	s := NewSlicer[int]()

	for i := 0; i < 1000; i++ {
		idx := s.Append(i)
		if int(idx) != i {
			t.Fatalf("expected index %d, got %d", i, idx)
		}
	}

	if s.Len() != 1000 {
		t.Fatalf("expected len 1000, got %d", s.Len())
	}

	for i := 0; i < 1000; i++ {
		v, ok := s.Get(uint64(i))
		if !ok || v != i {
			t.Fatalf("invalid value at %d: %v", i, v)
		}
	}
}

// Test concurrent appends.
func TestSlicerConcurrentAppend(t *testing.T) {
	s := NewSlicer[int]()
	var wg sync.WaitGroup

	workers := 8
	perWorker := 5000

	wg.Add(workers)

	for w := 0; w < workers; w++ {
		go func(base int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				s.Append(base + i)
			}
		}(w * perWorker)
	}

	wg.Wait()

	expected := uint64(workers * perWorker)
	if s.Len() != expected {
		t.Fatalf("expected len %d, got %d", expected, s.Len())
	}
}

// Test concurrent reads and writes.
func TestSlicerConcurrentReadWrite(t *testing.T) {
	s := NewSlicer[int]()
	var wg sync.WaitGroup

	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < 10000; i++ {
			s.Append(i)
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 10000; i++ {
			s.Len()
		}
	}()

	wg.Wait()
}

// Test snapshot correctness.
func TestSlicerSnapshot(t *testing.T) {
	s := NewSlicer[int]()

	for i := 0; i < 1000; i++ {
		s.Append(i)
	}

	snap := s.Snapshot()

	if len(snap) != 1000 {
		t.Fatalf("expected snapshot len 1000, got %d", len(snap))
	}

	// ensure all values exist
	seen := make(map[int]bool)
	for _, v := range snap {
		seen[v] = true
	}

	for i := 0; i < 1000; i++ {
		if !seen[i] {
			t.Fatalf("missing value %d in snapshot", i)
		}
	}
}

// Test range iteration.
func TestSlicerRange(t *testing.T) {
	s := NewSlicer[int]()

	for i := 0; i < 500; i++ {
		s.Append(i)
	}

	count := 0
	s.Range(func(_ uint64, v int) bool {
		count++
		return true
	})

	if count != 500 {
		t.Fatalf("expected 500 elements, got %d", count)
	}
}

// Test out-of-bounds access.
func TestSlicerGetOutOfBounds(t *testing.T) {
	s := NewSlicer[int]()

	if _, ok := s.Get(0); ok {
		t.Fatalf("expected false on empty slicer")
	}

	s.Append(1)

	if _, ok := s.Get(2); ok {
		t.Fatalf("expected false for out-of-bounds index")
	}
}

// Benchmark single-thread append performance.
func BenchmarkSlicerAppend(b *testing.B) {
	s := NewSlicer[int]()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Append(i)
	}
}

// Benchmark parallel append to measure contention behavior.
func BenchmarkSlicerAppendParallel(b *testing.B) {
	s := NewSlicer[int]()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			s.Append(i)
			i++
		}
	})
}

// Benchmark read performance using Get.
func BenchmarkSlicerGet(b *testing.B) {
	s := NewSlicer[int]()

	for i := 0; i < 1_000_000; i++ {
		s.Append(i)
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := uint64(0)
		for pb.Next() {
			s.Get(i % s.Len())
			i++
		}
	})
}

// Benchmark mixed read/write workload.
func BenchmarkSlicerMixed(b *testing.B) {
	s := NewSlicer[int]()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := uint64(0)
		for pb.Next() {
			if i%2 == 0 {
				s.Append(int(i))
			} else {
				s.Get(i % (s.Len() + 1))
			}
			i++
		}
	})
}

//
// ===== BASELINE COMPARISON =====
//

// mutexSlice is a naive thread-safe slice using a mutex.
type mutexSlice struct {
	mu sync.Mutex
	s  []int
}

// append safely adds an element.
func (m *mutexSlice) append(v int) {
	m.mu.Lock()
	m.s = append(m.s, v)
	m.mu.Unlock()
}

// get safely retrieves an element.
func (m *mutexSlice) get(i int) (int, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if i < 0 || i >= len(m.s) {
		return 0, false
	}
	return m.s[i], true
}

// Benchmark mutex-based slice append.
func BenchmarkMutexSliceAppend(b *testing.B) {
	m := &mutexSlice{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.append(i)
	}
}

// Benchmark mutex-based slice parallel append.
func BenchmarkMutexSliceAppendParallel(b *testing.B) {
	m := &mutexSlice{}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			m.append(i)
			i++
		}
	})
}

// Benchmark mutex-based slice read.
func BenchmarkMutexSliceGet(b *testing.B) {
	m := &mutexSlice{}

	for i := 0; i < 1_000_000; i++ {
		m.append(i)
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			m.get(i % len(m.s))
			i++
		}
	})
}
