package mappo

import (
	"hash/maphash"
	"unsafe"

	"github.com/cespare/xxhash/v2"
)

// Hasher provides consistent hashing for different key types.
type Hasher[K comparable] interface {
	Hash(key K) uint64
}

// StringHasher implements Hasher for string keys using xxhash.
type StringHasher struct{}

func (h StringHasher) Hash(key string) uint64 {
	return xxhash.Sum64String(key)
}

// IntHasher implements Hasher for int keys.
type IntHasher struct{}

func (h IntHasher) Hash(key int) uint64 {
	return uint64(key)
}

// Int64Hasher implements Hasher for int64 keys.
type Int64Hasher struct{}

func (h Int64Hasher) Hash(key int64) uint64 {
	return uint64(key)
}

// BytesHasher implements Hasher for byte slice keys using xxhash.
type BytesHasher struct{}

func (h BytesHasher) Hash(key []byte) uint64 {
	return xxhash.Sum64(key)
}

// MaphashHasher uses runtime maphash for generic hashing.
type MaphashHasher[K comparable] struct {
	seed maphash.Seed
}

func NewMaphashHasher[K comparable]() *MaphashHasher[K] {
	return &MaphashHasher[K]{seed: maphash.MakeSeed()}
}

func (h *MaphashHasher[K]) Hash(key K) uint64 {
	// Use unsafe to hash arbitrary comparable types via maphash
	// This is safe because maphash only reads the memory
	var str string
	switch any(key).(type) {
	case string:
		str = any(key).(string)
		return maphash.String(h.seed, str)
	case []byte:
		return maphash.Bytes(h.seed, any(key).([]byte))
	default:
		// For other types, use unsafe pointer to bytes
		// This works for comparable types (ints, structs, etc.)
		ptr := unsafe.Pointer(&key)
		size := unsafe.Sizeof(key)
		slice := unsafe.Slice((*byte)(ptr), size)
		return maphash.Bytes(h.seed, slice)
	}
}

// HashString returns a 32-bit hash for a string using FNV-1a.
func HashString(key string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(key); i++ {
		h ^= uint32(key[i])
		h *= 16777619
	}
	return h
}

// HashBytes returns a 32-bit hash for bytes using FNV-1a.
func HashBytes(key []byte) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(key); i++ {
		h ^= uint32(key[i])
		h *= 16777619
	}
	return h
}

// XxHash64 returns a 64-bit xxhash for a string.
func XxHash64(key string) uint64 {
	return xxhash.Sum64String(key)
}

// XxHash32 returns a 32-bit xxhash for a string.
func XxHash32(key string) uint32 {
	h64 := xxhash.Sum64String(key)
	return uint32(h64 ^ (h64 >> 32))
}

// XxHash64Bytes returns a 64-bit xxhash for bytes.
func XxHash64Bytes(key []byte) uint64 {
	return xxhash.Sum64(key)
}
