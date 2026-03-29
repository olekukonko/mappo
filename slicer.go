package mappo

import (
	"runtime"
	"sync/atomic"
)

// SlicerMode determines the concurrency safety level of the Slicer.
type SlicerMode int

const (
	// SlicerModeFast provides best performance (~3ns/op).
	// Uses per-element atomic state flags to satisfy race detector
	// while maintaining near-zero overhead.
	SlicerModeFast SlicerMode = iota

	// SlicerModeSafe provides full race safety with atomic.Pointer (~21ns/op).
	// Each Append allocates on heap. Use when maximum safety is required.
	SlicerModeSafe
)

// SlicerConfig allows configuration of Slicer behavior.
type SlicerConfig struct {
	Mode       SlicerMode
	ChunkSize  int
	ShardCount int
}

// DefaultSlicerConfig returns a config with sensible defaults.
func DefaultSlicerConfig() SlicerConfig {
	return SlicerConfig{
		Mode:      SlicerModeFast,
		ChunkSize: 1024,
	}
}

// slot represents a single element with atomic ready state.
// The ready flag provides the happens-before relationship needed
// for race detector satisfaction without heap allocation.
type slot[T any] struct {
	value T
	ready atomic.Bool
}

// chunk is the internal storage unit.
type chunk[T any] struct {
	slots []slot[T]
	// For SlicerModeSafe
	values []atomic.Pointer[T]

	count atomic.Uint32
	next  atomic.Pointer[chunk[T]]
}

// stripe isolates a write lane to reduce contention.
type stripe[T any] struct {
	head atomic.Pointer[chunk[T]]
	tail atomic.Pointer[chunk[T]]
}

// Slicer is a high-performance concurrent append-only slice.
type Slicer[T any] struct {
	stripes   []stripe[T]
	mask      uint64
	length    atomic.Uint64
	config    SlicerConfig
	chunkSize int
}

// NewSlicer creates a new concurrent slicer with default config.
func NewSlicer[T any]() *Slicer[T] {
	return NewSlicerWithConfig[T](DefaultSlicerConfig())
}

// NewSlicerWithConfig creates a new concurrent slicer with custom config.
func NewSlicerWithConfig[T any](config SlicerConfig) *Slicer[T] {
	if config.ChunkSize <= 0 {
		config.ChunkSize = 1024
	}

	shardCount := config.ShardCount
	if shardCount <= 0 {
		n := runtime.NumCPU()
		shardCount = 1
		for shardCount < n {
			shardCount <<= 1
		}
		if shardCount < 2 {
			shardCount = 2
		}
	}

	// Ensure power of 2
	if shardCount&(shardCount-1) != 0 {
		shardCount--
		shardCount |= shardCount >> 1
		shardCount |= shardCount >> 2
		shardCount |= shardCount >> 4
		shardCount |= shardCount >> 8
		shardCount |= shardCount >> 16
		shardCount++
	}

	// Update the config with the rounded shard count
	config.ShardCount = shardCount

	s := &Slicer[T]{
		stripes:   make([]stripe[T], shardCount),
		mask:      uint64(shardCount - 1),
		config:    config,
		chunkSize: config.ChunkSize,
	}

	for i := range s.stripes {
		c := s.newChunk()
		s.stripes[i].head.Store(c)
		s.stripes[i].tail.Store(c)
	}

	return s
}

func (s *Slicer[T]) newChunk() *chunk[T] {
	c := &chunk[T]{}
	if s.config.Mode == SlicerModeSafe {
		c.values = make([]atomic.Pointer[T], s.chunkSize)
	} else {
		c.slots = make([]slot[T], s.chunkSize)
	}
	return c
}

// Append adds a value and returns its global index.
func (s *Slicer[T]) Append(val T) uint64 {
	idx := s.length.Add(1) - 1
	stripe := &s.stripes[idx&s.mask]

	for {
		tail := stripe.tail.Load()
		pos := tail.count.Add(1) - 1

		if int(pos) < s.chunkSize {
			if s.config.Mode == SlicerModeSafe {
				p := new(T)
				*p = val
				tail.values[pos].Store(p)
			} else {
				// Write value then mark ready.
				// The ready.Store provides release semantics ensuring
				// the value write is visible to Range.
				tail.slots[pos].value = val
				tail.slots[pos].ready.Store(true)
			}
			return idx
		}

		// pos >= chunkSize: this chunk is full. Allocate a new one,
		// write into slot 0, then try to link it.
		newChunk := s.newChunk()
		if s.config.Mode == SlicerModeSafe {
			p := new(T)
			*p = val
			newChunk.values[0].Store(p)
		} else {
			newChunk.slots[0].value = val
			newChunk.slots[0].ready.Store(true)
		}
		newChunk.count.Store(1)

		if tail.next.CompareAndSwap(nil, newChunk) {
			stripe.tail.CompareAndSwap(tail, newChunk)
			return idx
		}

		// CAS lost: another goroutine already linked a new chunk.
		// Roll back the over-increment we added to the old chunk so that
		// Get's chunk-walk arithmetic is never corrupted by a phantom slot.
		tail.count.Add(^uint32(0)) // atomic decrement by 1
		stripe.tail.CompareAndSwap(tail, tail.next.Load())
	}
}

// Get returns the value at index if it exists.
func (s *Slicer[T]) Get(index uint64) (T, bool) {
	var zero T

	if index >= s.length.Load() {
		return zero, false
	}

	stripe := &s.stripes[index&s.mask]
	c := stripe.head.Load()
	remaining := index / uint64(len(s.stripes))

	for c != nil {
		count := uint64(c.count.Load())
		// Clamp for the same reason as in Range: count can be
		// transiently over-committed by concurrent Append callers.
		if int(count) > s.chunkSize {
			count = uint64(s.chunkSize)
		}
		if remaining < count {
			if int(remaining) >= s.chunkSize {
				return zero, false
			}
			if s.config.Mode == SlicerModeSafe {
				ptr := c.values[remaining].Load()
				if ptr == nil {
					return zero, false
				}
				return *ptr, true
			}
			// Check ready flag first (acquire semantics)
			if !c.slots[remaining].ready.Load() {
				return zero, false
			}
			return c.slots[remaining].value, true
		}
		remaining -= count
		c = c.next.Load()
	}

	return zero, false
}

// Len returns the total number of elements.
func (s *Slicer[T]) Len() uint64 {
	return s.length.Load()
}

// Range iterates over all elements in approximate order.
func (s *Slicer[T]) Range(fn func(uint64, T) bool) {
	var base uint64

	for i := range s.stripes {
		c := s.stripes[i].head.Load()
		local := uint64(0)

		for c != nil {
			count := c.count.Load()
			// count is incremented optimistically before the bounds check
			// in Append, so it can exceed chunkSize by the number of
			// concurrent writers that lost the CAS race and are retrying.
			// Clamp to chunkSize so we never index out of bounds.
			if int(count) > s.chunkSize {
				count = uint32(s.chunkSize)
			}

			for j := uint32(0); j < count; j++ {
				var val T
				if s.config.Mode == SlicerModeSafe {
					ptr := c.values[j].Load()
					if ptr == nil {
						continue
					}
					val = *ptr
				} else {
					// Load ready flag first (acquire semantics).
					// This establishes happens-before with the Append
					// that wrote this slot, satisfying the race detector.
					if !c.slots[j].ready.Load() {
						continue
					}
					val = c.slots[j].value
				}
				globalIdx := base + local
				if !fn(globalIdx, val) {
					return
				}
				local++
			}

			c = c.next.Load()
		}

		base += local
	}
}

// Snapshot returns a copy of all elements.
func (s *Slicer[T]) Snapshot() []T {
	total := s.Len()
	if total == 0 {
		return nil
	}

	out := make([]T, 0, total)
	s.Range(func(_ uint64, v T) bool {
		out = append(out, v)
		return true
	})
	return out
}

// Config returns the slicer configuration.
func (s *Slicer[T]) Config() SlicerConfig {
	return s.config
}
