package mappo

import (
	"fmt"
	"hash/fnv"

	"github.com/cespare/xxhash/v2"
)

// Hasher provides consistent hashing for different key types.
type Hasher interface {
	Hash(key any) uint64
}

// FNVHasher implements Hasher using FNV-1a.
type FNVHasher struct{}

// Hash returns FNV-1a hash of the key.
func (h FNVHasher) Hash(key any) uint64 {
	f := fnv.New64a()
	f.Write([]byte(fmt.Sprint(key)))
	return f.Sum64()
}

// XXHasher implements Hasher using xxhash.
type XXHasher struct{}

// Hash returns xxhash of the key.
func (h XXHasher) Hash(key any) uint64 {
	return xxhash.Sum64String(fmt.Sprint(key))
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

// XxHash32 returns a 32-bit xxhash for a string.
func XxHash32(key string) uint32 {
	h64 := xxhash.Sum64String(key)
	return uint32(h64 ^ (h64 >> 32))
}
