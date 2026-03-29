package mappo

import (
	"runtime"
	"sync/atomic"
)

// chunkSize defines number of elements per chunk.
// Tune based on cache behavior and workload.
const chunkSize = 1024

// chunk is a fixed-size append-only block.
type chunk[T any] struct {
	data  [chunkSize]T
	count atomic.Uint32
	next  atomic.Pointer[chunk[T]]
}

// stripe isolates a write lane to reduce contention.
// Each stripe maintains its own chunk chain.
type stripe[T any] struct {
	head atomic.Pointer[chunk[T]]
	tail atomic.Pointer[chunk[T]]
}

// Slicer is a high-performance concurrent append-only slice.
// It uses striped chunking to minimize contention under heavy writes.
type Slicer[T any] struct {
	stripes []stripe[T]
	mask    uint64
	length  atomic.Uint64
}

// NewSlicer creates a new concurrent slicer.
// It initializes stripes based on CPU count (rounded to power of two).
func NewSlicer[T any]() *Slicer[T] {
	n := runtime.NumCPU()

	size := 1
	for size < n {
		size <<= 1
	}
	if size < 2 {
		size = 2
	}

	s := &Slicer[T]{
		stripes: make([]stripe[T], size),
		mask:    uint64(size - 1),
	}

	for i := range s.stripes {
		c := &chunk[T]{}
		s.stripes[i].head.Store(c)
		s.stripes[i].tail.Store(c)
	}

	return s
}

// Append adds a value and returns its global index.
// It distributes writes across stripes to reduce contention.
func (s *Slicer[T]) Append(val T) uint64 {
	idx := s.length.Add(1) - 1

	stripe := &s.stripes[idx&s.mask]

	for {
		tail := stripe.tail.Load()

		pos := tail.count.Add(1) - 1
		if pos < chunkSize {
			tail.data[pos] = val
			return idx
		}

		newChunk := &chunk[T]{}
		newChunk.data[0] = val
		newChunk.count.Store(1)

		if tail.next.CompareAndSwap(nil, newChunk) {
			stripe.tail.CompareAndSwap(tail, newChunk)
			return idx
		}

		stripe.tail.CompareAndSwap(tail, tail.next.Load())
	}
}

// Get returns the value at index if it exists.
// It maps the index to a stripe and walks its chunk chain.
func (s *Slicer[T]) Get(index uint64) (T, bool) {
	var zero T

	if index >= s.length.Load() {
		return zero, false
	}

	stripe := &s.stripes[index&s.mask]

	// IMPORTANT: we cannot trust perfect distribution
	// so we walk and subtract instead of direct division
	c := stripe.head.Load()
	remaining := index / uint64(len(s.stripes))

	for c != nil {
		count := uint64(c.count.Load())

		if remaining < count {
			// extra safety guard
			if remaining >= uint64(len(c.data)) {
				return zero, false
			}
			return c.data[remaining], true
		}

		remaining -= count
		c = c.next.Load()
	}

	return zero, false
}

// Len returns the total number of elements.
// It is a monotonic counter updated atomically.
func (s *Slicer[T]) Len() uint64 {
	return s.length.Load()
}

// Range iterates over all elements in approximate order.
// It walks each stripe independently without locking.
func (s *Slicer[T]) Range(fn func(uint64, T) bool) {
	var base uint64

	for i := range s.stripes {
		c := s.stripes[i].head.Load()
		local := uint64(0)

		for c != nil {
			count := c.count.Load()

			for j := uint32(0); j < count; j++ {
				globalIdx := base + local
				if !fn(globalIdx, c.data[j]) {
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
// It flattens stripes into a single slice.
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
