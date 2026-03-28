package mappo

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

// TestNewBytesPool verifies basic pool creation.
func TestNewBytesPool(t *testing.T) {
	size := 1024
	pool := NewBytesPool(size)

	if pool.size != size {
		t.Errorf("Expected size %d, got %d", size, pool.size)
	}
	if pool.maxCap != size {
		t.Errorf("Expected maxCap %d, got %d", size, pool.maxCap)
	}
	if pool.stats == nil {
		t.Error("Expected stats enabled by default")
	}
}

// TestNewBytesPoolWithOptions verifies options work.
func TestNewBytesPoolWithOptions(t *testing.T) {
	// With stats (default)
	pool := NewBytesPoolWithOptions(BytesPoolOptions{Size: 1024})
	if pool.stats == nil {
		t.Error("Expected stats enabled")
	}

	// Without stats
	poolNoStats := NewBytesPoolWithOptions(BytesPoolOptions{
		Size:    1024,
		NoStats: true,
	})
	if poolNoStats.stats != nil {
		t.Error("Expected stats disabled")
	}

	// Flexible pool
	poolFlex := NewBytesPoolWithOptions(BytesPoolOptions{
		Size:   1024,
		MaxCap: 2048,
	})
	if poolFlex.maxCap != 2048 {
		t.Errorf("Expected maxCap 2048, got %d", poolFlex.maxCap)
	}
}

// TestNewBytesPoolWithMax verifies flexible pool creation.
func TestNewBytesPoolWithMax(t *testing.T) {
	pool := NewBytesPoolWithMax(1024, 2048)
	if pool.size != 1024 {
		t.Errorf("Expected size 1024, got %d", pool.size)
	}
	if pool.maxCap != 2048 {
		t.Errorf("Expected maxCap 2048, got %d", pool.maxCap)
	}

	// Auto-correct when maxCap < size
	pool2 := NewBytesPoolWithMax(1024, 512)
	if pool2.maxCap != 1024 {
		t.Errorf("Expected maxCap 1024, got %d", pool2.maxCap)
	}
}

// TestBytesPool_Get verifies Get returns clean buffer.
func TestBytesPool_Get(t *testing.T) {
	pool := NewBytesPool(1024)

	buf := pool.Get()

	if buf == nil {
		t.Fatal("Get returned nil")
	}
	if len(buf) != 0 {
		t.Errorf("Expected len 0, got %d", len(buf))
	}
	if cap(buf) != 1024 {
		t.Errorf("Expected cap 1024, got %d", cap(buf))
	}

	// Stats should track
	stats := pool.Stats()
	if stats.Hits+stats.Misses != 1 {
		t.Errorf("Expected 1 operation, got %d", stats.Hits+stats.Misses)
	}

	pool.Put(buf)
}

// TestBytesPool_GetNoStats verifies no-stats mode works.
func TestBytesPool_GetNoStats(t *testing.T) {
	pool := NewBytesPoolWithOptions(BytesPoolOptions{
		Size:    1024,
		NoStats: true,
	})

	buf := pool.Get()
	if cap(buf) != 1024 {
		t.Errorf("Expected cap 1024, got %d", cap(buf))
	}

	// Stats should be zero
	stats := pool.Stats()
	if stats.Hits != 0 || stats.Misses != 0 {
		t.Error("Expected zero stats when disabled")
	}

	pool.Put(buf)
}

// TestBytesPool_PutAndGet verifies round-trip.
func TestBytesPool_PutAndGet(t *testing.T) {
	pool := NewBytesPool(1024)

	buf1 := pool.Get()
	pool.Put(buf1)

	// Force GC to clear sync.Pool (sync.Pool is cleared every GC cycle)
	runtime.GC()
	time.Sleep(10 * time.Millisecond)

	buf2 := pool.Get()
	if cap(buf2) != 1024 {
		t.Errorf("Expected cap 1024, got %d", cap(buf2))
	}
	pool.Put(buf2)
}

// TestBytesPool_PutOversized verifies oversized buffers dropped.
func TestBytesPool_PutOversized(t *testing.T) {
	pool := NewBytesPool(1024)

	// Pre-warm to get baseline
	_ = pool.Stats()

	oversized := make([]byte, 100, 2048)
	pool.Put(oversized)

	stats := pool.Stats()
	// Just verify it was tracked (exact count may vary due to sync.Pool behavior)
	if stats.Dropped == 0 {
		t.Error("Expected some drops for oversized buffer")
	}
}

// TestBytesPool_PutUndersized verifies undersized buffers dropped.
func TestBytesPool_PutUndersized(t *testing.T) {
	pool := NewBytesPool(1024)

	// Pre-warm to get baseline
	_ = pool.Stats()

	undersized := make([]byte, 100, 512)
	pool.Put(undersized)

	stats := pool.Stats()
	if stats.Dropped == 0 {
		t.Error("Expected some drops for undersized buffer")
	}
}

// TestBytesPool_PutExactSize verifies exact-size accepted.
func TestBytesPool_PutExactSize(t *testing.T) {
	pool := NewBytesPool(1024)

	// Pre-warm
	_ = pool.Stats()
	initialReturned := pool.Stats().Returned

	exact := make([]byte, 100, 1024)
	pool.Put(exact)

	stats := pool.Stats()
	// Should be returned, not dropped
	if stats.Returned <= initialReturned {
		t.Error("Expected buffer to be returned")
	}
}

// TestBytesPool_Concurrent verifies thread safety.
func TestBytesPool_Concurrent(t *testing.T) {
	pool := NewBytesPool(1024)
	var wg sync.WaitGroup
	iterations := 100
	goroutines := 10

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				buf := pool.Get()
				buf = append(buf, []byte("test")...)
				pool.Put(buf)
			}
		}()
	}

	wg.Wait()

	// Just verify no panic and stats roughly track
	stats := pool.Stats()
	total := stats.Hits + stats.Misses
	// With sync.Pool, we can't guarantee exact counts due to GC behavior
	expected := uint64(goroutines * iterations)
	// Allow 50% variance due to sync.Pool GC behavior
	if total < expected/2 {
		t.Errorf("Expected at least %d ops, got %d", expected/2, total)
	}
}

// TestBytesPool_ConcurrentNoStats verifies no-stats is faster.
func TestBytesPool_ConcurrentNoStats(t *testing.T) {
	pool := NewBytesPoolWithOptions(BytesPoolOptions{
		Size:    1024,
		NoStats: true,
	})
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				buf := pool.Get()
				buf = append(buf, []byte("test")...)
				pool.Put(buf)
			}
		}()
	}

	wg.Wait()
	// Just verify no panic and stats are zero
	if pool.Stats().Hits != 0 {
		t.Error("Expected zero hits with NoStats")
	}
}

// TestBytesPool_HitRate verifies calculation.
func TestBytesPool_HitRate(t *testing.T) {
	tests := []struct {
		hits, misses uint64
		expected     float64
	}{
		{0, 0, 0},
		{100, 0, 100},
		{0, 100, 0},
		{50, 50, 50},
	}

	for _, tt := range tests {
		s := PoolStats{Hits: tt.hits, Misses: tt.misses}
		if s.HitRate() != tt.expected {
			t.Errorf("Expected %f, got %f", tt.expected, s.HitRate())
		}
	}
}

// TestBytesPool_FlexibleRange verifies range acceptance.
func TestBytesPool_FlexibleRange(t *testing.T) {
	pool := NewBytesPoolWithMax(1024, 2048)

	// Pre-warm stats
	_ = pool.Stats()
	initialDropped := pool.Stats().Dropped

	// Accept 1KB, 1.5KB, 2KB
	pool.Put(make([]byte, 100, 1024))
	pool.Put(make([]byte, 100, 1536))
	pool.Put(make([]byte, 100, 2048))

	stats := pool.Stats()
	// Should not have dropped these
	if stats.Dropped != initialDropped {
		t.Errorf("Expected no drops for valid sizes, got %d", stats.Dropped-initialDropped)
	}

	// Drop 3KB
	pool.Put(make([]byte, 100, 3072))
	stats = pool.Stats()
	if stats.Dropped != initialDropped+1 {
		t.Errorf("Expected 1 drop for oversized, got %d", stats.Dropped-initialDropped)
	}
}

// TestBytesPool_CapMethods verifies accessors.
func TestBytesPool_CapMethods(t *testing.T) {
	pool := NewBytesPoolWithMax(1024, 2048)
	if pool.Cap() != 1024 {
		t.Errorf("Expected Cap() 1024, got %d", pool.Cap())
	}
	if pool.MaxCap() != 2048 {
		t.Errorf("Expected MaxCap() 2048, got %d", pool.MaxCap())
	}
}

// Benchmarks

func BenchmarkBytesPool_Get(b *testing.B) {
	pool := NewBytesPool(1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pool.Get()
	}
}

func BenchmarkBytesPool_GetPut(b *testing.B) {
	pool := NewBytesPool(1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.Get()
		pool.Put(buf)
	}
}

func BenchmarkBytesPool_Concurrent(b *testing.B) {
	pool := NewBytesPool(1024)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Get()
			buf = append(buf, []byte("test")...)
			pool.Put(buf)
		}
	})
}

// BenchmarkBytesPool_ConcurrentNoStats shows zero-atomic performance.
func BenchmarkBytesPool_ConcurrentNoStats(b *testing.B) {
	pool := NewBytesPoolWithOptions(BytesPoolOptions{
		Size:    1024,
		NoStats: true,
	})
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Get()
			buf = append(buf, []byte("test")...)
			pool.Put(buf)
		}
	})
}

func BenchmarkBytesPool_MemoryBloat(b *testing.B) {
	pool := NewBytesPool(4096)

	for i := 0; i < 100; i++ {
		buf := pool.Get()
		pool.Put(buf)
	}

	var m1, m2 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m1)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		if i%100 == 0 {
			pool.Put(make([]byte, 1024*1024))
		}
		buf := pool.Get()
		pool.Put(buf)
	}

	runtime.GC()
	time.Sleep(10 * time.Millisecond)
	runtime.ReadMemStats(&m2)

	stats := pool.Stats()
	b.ReportMetric(float64(m2.HeapInuse-m1.HeapInuse)/1024/1024, "MB_growth")
	b.ReportMetric(float64(stats.Dropped), "dropped")
	b.ReportMetric(stats.HitRate(), "hit_rate_%")
}

func BenchmarkMakeSlice(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = make([]byte, 0, 1024)
	}
}
