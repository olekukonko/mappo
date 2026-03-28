package mappo

import (
	"sync"
	"sync/atomic"
)

// PoolStats holds atomic statistics for BytesPool.
// Only populated if stats enabled.
type PoolStats struct {
	Hits     uint64 // Successful pool retrievals
	Misses   uint64 // Allocations when pool empty
	Returned uint64 // Buffers returned to pool
	Dropped  uint64 // Buffers rejected (wrong size)
}

// HitRate returns cache hit rate as percentage.
func (s PoolStats) HitRate() float64 {
	total := s.Hits + s.Misses
	if total == 0 {
		return 0
	}
	return float64(s.Hits) / float64(total) * 100
}

// poolStats holds atomic counters internally.
type poolStats struct {
	hits     atomic.Uint64
	misses   atomic.Uint64
	returned atomic.Uint64
	dropped  atomic.Uint64
}

// BytesPool provides sync.Pool-based buffer reuse with optional size enforcement.
// Zero value is NOT usable - use NewBytesPool or NewBytesPoolWithOptions.
type BytesPool struct {
	pool   sync.Pool
	size   int
	maxCap int
	stats  *poolStats // nil = stats disabled (zero overhead)
}

// BytesPoolOptions configures BytesPool.
type BytesPoolOptions struct {
	Size    int  // Target buffer size (required)
	MaxCap  int  // Max capacity allowed (0 = exact size only)
	NoStats bool // Disable stats tracking for zero atomic overhead
}

// NewBytesPool creates a strict pool for exact-size buffers.
// Use this for maximum performance when all buffers are uniform.
func NewBytesPool(size int) *BytesPool {
	return NewBytesPoolWithOptions(BytesPoolOptions{
		Size:   size,
		MaxCap: size,
	})
}

// NewBytesPoolWithMax creates a flexible pool accepting buffers up to maxCap.
// Slightly slower due to capacity range check.
func NewBytesPoolWithMax(size, maxCap int) *BytesPool {
	return NewBytesPoolWithOptions(BytesPoolOptions{
		Size:   size,
		MaxCap: maxCap,
	})
}

// NewBytesPoolWithOptions creates a pool with full configuration.
func NewBytesPoolWithOptions(opt BytesPoolOptions) *BytesPool {
	if opt.Size <= 0 {
		panic("BytesPool: Size must be > 0")
	}
	maxCap := opt.MaxCap
	if maxCap < opt.Size {
		maxCap = opt.Size
	}

	p := &BytesPool{
		size:   opt.Size,
		maxCap: maxCap,
		pool: sync.Pool{
			New: func() any {
				return make([]byte, opt.Size)
			},
		},
	}

	if !opt.NoStats {
		p.stats = &poolStats{}
	}

	return p
}

// Get retrieves a buffer from the pool or allocates new.
// Returns slice with len=0, cap=size.
// Fast path: no atomic operations when stats disabled.
func (p *BytesPool) Get() []byte {
	if v := p.pool.Get(); v != nil {
		// Fast path: pool had a buffer
		if p.stats != nil {
			p.stats.hits.Add(1)
		}
		buf := v.([]byte)
		return buf[:0]
	}
	// Slow path: allocation
	if p.stats != nil {
		p.stats.misses.Add(1)
	}
	return make([]byte, 0, p.size)
}

// Put returns a buffer to the pool.
// Fast path: no atomic operations when stats disabled and exact size match.
func (p *BytesPool) Put(buf []byte) {
	// Size check first (branch predictor friendly for exact matches)
	c := cap(buf)
	if c != p.size {
		// Slow path: range check for flexible pools
		if c < p.size || c > p.maxCap {
			// Drop wrong-sized buffer
			if p.stats != nil {
				p.stats.returned.Add(1)
				p.stats.dropped.Add(1)
			}
			return
		}
	}

	// Fast path: accepted buffer
	if p.stats != nil {
		p.stats.returned.Add(1)
	}
	p.pool.Put(buf[:c])
}

// Stats returns current pool statistics.
// Returns zero values if stats disabled.
func (p *BytesPool) Stats() PoolStats {
	if p.stats == nil {
		return PoolStats{}
	}
	return PoolStats{
		Hits:     p.stats.hits.Load(),
		Misses:   p.stats.misses.Load(),
		Returned: p.stats.returned.Load(),
		Dropped:  p.stats.dropped.Load(),
	}
}

// Cap returns the target buffer capacity.
func (p *BytesPool) Cap() int {
	return p.size
}

// MaxCap returns the maximum allowed capacity.
func (p *BytesPool) MaxCap() int {
	return p.maxCap
}
