package mappo

import (
	"hash/maphash"
	"testing"
	"unsafe"
)

func TestMakeHasher(t *testing.T) {
	seed := maphash.MakeSeed()

	t.Run("string", func(t *testing.T) {
		hasher := makeHasher[string]()
		h1 := hasher("test", seed)
		h2 := hasher("test", seed)
		if h1 != h2 {
			t.Error("inconsistent hash for same key")
		}
		if h1 == hasher("other", seed) {
			t.Error("same hash for different keys")
		}
	})

	t.Run("int", func(t *testing.T) {
		hasher := makeHasher[int]()
		h1 := hasher(42, seed)
		h2 := hasher(42, seed)
		if h1 != h2 {
			t.Error("inconsistent hash for same key")
		}
		if h1 == hasher(43, seed) {
			t.Error("same hash for different keys")
		}
		// Check seed independence
		otherSeed := maphash.MakeSeed()
		if h1 == hasher(42, otherSeed) {
			t.Error("hash should differ with different seed")
		}
	})

	t.Run("int64", func(t *testing.T) {
		hasher := makeHasher[int64]()
		h1 := hasher(42, seed)
		h2 := hasher(42, seed)
		if h1 != h2 {
			t.Error("inconsistent hash for same key")
		}
		if h1 == hasher(43, seed) {
			t.Error("same hash for different keys")
		}
	})

	t.Run("uint64", func(t *testing.T) {
		hasher := makeHasher[uint64]()
		h1 := hasher(42, seed)
		h2 := hasher(42, seed)
		if h1 != h2 {
			t.Error("inconsistent hash for same key")
		}
		if h1 == hasher(43, seed) {
			t.Error("same hash for different keys")
		}
	})

	t.Run("default (struct)", func(t *testing.T) {
		type testKey struct {
			a int
			b string
		}
		hasher := makeHasher[testKey]()
		key := testKey{1, "a"}
		h1 := hasher(key, seed)
		h2 := hasher(key, seed)
		if h1 != h2 {
			t.Error("inconsistent hash for same key")
		}
		if h1 == hasher(testKey{2, "b"}, seed) {
			t.Error("same hash for different keys")
		}
		// Check memory representation
		ptr := unsafe.Pointer(&key)
		size := unsafe.Sizeof(key)
		slice := unsafe.Slice((*byte)(ptr), size)
		expected := maphash.Bytes(seed, slice)
		if h1 != expected {
			t.Error("fallback hash mismatch")
		}
	})
}

func BenchmarkMakeHasherString(b *testing.B) {
	hasher := makeHasher[string]()
	seed := maphash.MakeSeed()
	keys := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		keys[i] = string([]byte{byte(i)})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hasher(keys[i%1000], seed)
	}
}

func BenchmarkMakeHasherInt(b *testing.B) {
	hasher := makeHasher[int]()
	seed := maphash.MakeSeed()
	keys := make([]int, 1000)
	for i := 0; i < 1000; i++ {
		keys[i] = i
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hasher(keys[i%1000], seed)
	}
}

func BenchmarkMakeHasherInt64(b *testing.B) {
	hasher := makeHasher[int64]()
	seed := maphash.MakeSeed()
	keys := make([]int64, 1000)
	for i := 0; i < 1000; i++ {
		keys[i] = int64(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hasher(keys[i%1000], seed)
	}
}

func BenchmarkMakeHasherUint64(b *testing.B) {
	hasher := makeHasher[uint64]()
	seed := maphash.MakeSeed()
	keys := make([]uint64, 1000)
	for i := 0; i < 1000; i++ {
		keys[i] = uint64(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hasher(keys[i%1000], seed)
	}
}

func BenchmarkMakeHasherDefault(b *testing.B) {
	type testKey struct {
		a int
		b string
	}
	hasher := makeHasher[testKey]()
	seed := maphash.MakeSeed()
	keys := make([]testKey, 1000)
	for i := 0; i < 1000; i++ {
		keys[i] = testKey{i, string([]byte{byte(i)})}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = hasher(keys[i%1000], seed)
	}
}
