package mappo

import (
	"hash/maphash"
	"unsafe"
)

// makeHasher creates a type-specific hash function.
func makeHasher[K comparable]() func(K, maphash.Seed) uint64 {
	var zero K
	switch any(zero).(type) {
	case string:
		return func(k K, seed maphash.Seed) uint64 {
			return maphash.String(seed, any(k).(string))
		}
	case int:
		return func(k K, seed maphash.Seed) uint64 {
			h := uint64(any(k).(int))
			return maphash.Bytes(seed, (*[8]byte)(unsafe.Pointer(&h))[:])
		}
	case int64:
		return func(k K, seed maphash.Seed) uint64 {
			h := uint64(any(k).(int64))
			return maphash.Bytes(seed, (*[8]byte)(unsafe.Pointer(&h))[:])
		}
	case uint64:
		return func(k K, seed maphash.Seed) uint64 {
			h := any(k).(uint64)
			return maphash.Bytes(seed, (*[8]byte)(unsafe.Pointer(&h))[:])
		}
	default:
		// Fallback
		return func(k K, seed maphash.Seed) uint64 {
			ptr := unsafe.Pointer(&k)
			size := unsafe.Sizeof(k)
			slice := unsafe.Slice((*byte)(ptr), size)
			return maphash.Bytes(seed, slice)
		}
	}
}
