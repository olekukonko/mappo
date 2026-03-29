package mappo

import (
	"sync"
	"testing"
)

// Test that default NewSlicer works (backward compatibility)
func TestSlicer_DefaultNewSlicer(t *testing.T) {
	s := NewSlicer[int]()

	idx := s.Append(42)
	if idx != 0 {
		t.Errorf("expected index 0, got %d", idx)
	}

	val, ok := s.Get(0)
	if !ok || val != 42 {
		t.Errorf("expected 42, got %d, ok=%v", val, ok)
	}

	// Should default to Standard mode
	cfg := s.Config()
	if cfg.Mode != SlicerModeFast {
		t.Errorf("expected default ModeFast, got %v", cfg.Mode)
	}
}

// Test both modes comprehensively
func TestSlicer_BothModes(t *testing.T) {
	tests := []struct {
		name string
		mode SlicerMode
	}{
		{"Standard", SlicerModeFast},
		{"Atomic", SlicerModeSafe},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSlicerWithConfig[int](SlicerConfig{Mode: tt.mode})

			// Basic append and get
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
		})
	}
}

// Test concurrent appends for both modes
func TestSlicer_ConcurrentAppend(t *testing.T) {
	tests := []struct {
		name string
		mode SlicerMode
	}{
		{"Standard", SlicerModeFast},
		{"Atomic", SlicerModeSafe},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSlicerWithConfig[int](SlicerConfig{Mode: tt.mode})
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
		})
	}
}

// Test concurrent reads and writes for both modes
func TestSlicer_ConcurrentReadWrite(t *testing.T) {
	tests := []struct {
		name string
		mode SlicerMode
	}{
		{"Standard", SlicerModeFast},
		{"Atomic", SlicerModeSafe},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSlicerWithConfig[int](SlicerConfig{Mode: tt.mode})
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
		})
	}
}

// Test snapshot correctness for both modes
func TestSlicer_Snapshot(t *testing.T) {
	tests := []struct {
		name string
		mode SlicerMode
	}{
		{"Standard", SlicerModeFast},
		{"Atomic", SlicerModeSafe},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSlicerWithConfig[int](SlicerConfig{Mode: tt.mode})

			for i := 0; i < 1000; i++ {
				s.Append(i)
			}

			snap := s.Snapshot()

			if len(snap) != 1000 {
				t.Fatalf("expected snapshot len 1000, got %d", len(snap))
			}

			seen := make(map[int]bool)
			for _, v := range snap {
				seen[v] = true
			}

			for i := 0; i < 1000; i++ {
				if !seen[i] {
					t.Fatalf("missing value %d in snapshot", i)
				}
			}
		})
	}
}

// Test range iteration for both modes
func TestSlicer_Range(t *testing.T) {
	tests := []struct {
		name string
		mode SlicerMode
	}{
		{"Standard", SlicerModeFast},
		{"Atomic", SlicerModeSafe},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSlicerWithConfig[int](SlicerConfig{Mode: tt.mode})

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
		})
	}
}

// Test out-of-bounds access for both modes
func TestSlicer_GetOutOfBounds(t *testing.T) {
	tests := []struct {
		name string
		mode SlicerMode
	}{
		{"Standard", SlicerModeFast},
		{"Atomic", SlicerModeSafe},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSlicerWithConfig[int](SlicerConfig{Mode: tt.mode})

			if _, ok := s.Get(0); ok {
				t.Fatalf("expected false on empty slicer")
			}

			s.Append(1)

			if _, ok := s.Get(2); ok {
				t.Fatalf("expected false for out-of-bounds index")
			}
		})
	}
}

// Test custom configuration
func TestSlicer_CustomConfig(t *testing.T) {
	s := NewSlicerWithConfig[int](SlicerConfig{
		Mode:       SlicerModeSafe,
		ChunkSize:  512,
		ShardCount: 4,
	})

	cfg := s.Config()
	if cfg.Mode != SlicerModeSafe {
		t.Errorf("expected ModeSafe, got %v", cfg.Mode)
	}
	if cfg.ChunkSize != 512 {
		t.Errorf("expected ChunkSize 512, got %d", cfg.ChunkSize)
	}
	if cfg.ShardCount != 4 {
		t.Errorf("expected ShardCount 4, got %d", cfg.ShardCount)
	}

	// Verify it works
	for i := 0; i < 10000; i++ {
		s.Append(i)
	}
	if s.Len() != 10000 {
		t.Errorf("expected len 10000, got %d", s.Len())
	}
}

// Test default config values
func TestSlicer_DefaultConfig(t *testing.T) {
	cfg := DefaultSlicerConfig()
	if cfg.Mode != SlicerModeFast {
		t.Errorf("expected default ModeFast, got %v", cfg.Mode)
	}
	if cfg.ChunkSize != 1024 {
		t.Errorf("expected default ChunkSize 1024, got %d", cfg.ChunkSize)
	}
	if cfg.ShardCount != 0 {
		t.Errorf("expected default ShardCount 0 (auto), got %d", cfg.ShardCount)
	}
}

// Test ShardCount power-of-2 rounding
func TestSlicer_ShardCountRounding(t *testing.T) {
	// Test that non-power-of-2 gets rounded up
	s := NewSlicerWithConfig[int](SlicerConfig{
		Mode:       SlicerModeFast,
		ShardCount: 5, // Should round to 8
	})

	cfg := s.Config()
	if cfg.ShardCount != 8 {
		t.Errorf("expected ShardCount rounded to 8, got %d", cfg.ShardCount)
	}
}

// Benchmark Standard mode (fast)
func BenchmarkSlicerAppend_Standard(b *testing.B) {
	s := NewSlicerWithConfig[int](SlicerConfig{Mode: SlicerModeFast})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Append(i)
	}
}

func BenchmarkSlicerAppendParallel_Standard(b *testing.B) {
	s := NewSlicerWithConfig[int](SlicerConfig{Mode: SlicerModeFast})
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			s.Append(i)
			i++
		}
	})
}

// Benchmark Atomic mode (race-safe)
func BenchmarkSlicerAppend_Atomic(b *testing.B) {
	s := NewSlicerWithConfig[int](SlicerConfig{Mode: SlicerModeSafe})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Append(i)
	}
}

func BenchmarkSlicerAppendParallel_Atomic(b *testing.B) {
	s := NewSlicerWithConfig[int](SlicerConfig{Mode: SlicerModeSafe})
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			s.Append(i)
			i++
		}
	})
}

// Benchmark Get for both modes
func BenchmarkSlicerGet_Standard(b *testing.B) {
	s := NewSlicerWithConfig[int](SlicerConfig{Mode: SlicerModeFast})
	for i := 0; i < 1_000_000; i++ {
		s.Append(i)
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := uint64(0)
		for pb.Next() {
			s.Get(i % 1_000_000)
			i++
		}
	})
}

func BenchmarkSlicerGet_Atomic(b *testing.B) {
	s := NewSlicerWithConfig[int](SlicerConfig{Mode: SlicerModeSafe})
	for i := 0; i < 1_000_000; i++ {
		s.Append(i)
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := uint64(0)
		for pb.Next() {
			s.Get(i % 1_000_000)
			i++
		}
	})
}

// Benchmark mixed workload
func BenchmarkSlicerMixed_Standard(b *testing.B) {
	s := NewSlicerWithConfig[int](SlicerConfig{Mode: SlicerModeFast})
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

func BenchmarkSlicerMixed_Atomic(b *testing.B) {
	s := NewSlicerWithConfig[int](SlicerConfig{Mode: SlicerModeSafe})
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

// Baseline comparison with mutex

type mutexSlice struct {
	mu sync.Mutex
	s  []int
}

func (m *mutexSlice) append(v int) {
	m.mu.Lock()
	m.s = append(m.s, v)
	m.mu.Unlock()
}

func (m *mutexSlice) get(i int) (int, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if i < 0 || i >= len(m.s) {
		return 0, false
	}
	return m.s[i], true
}

func BenchmarkMutexSliceAppend(b *testing.B) {
	m := &mutexSlice{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.append(i)
	}
}

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
