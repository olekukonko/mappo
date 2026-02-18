// hash_test.go
package mappo

import "testing"

func TestFNVHasher(t *testing.T) {
	h := FNVHasher{}
	if h.Hash("test") == 0 {
		t.Error("expected non-zero hash")
	}
	if h.Hash(42) != h.Hash(42) {
		t.Error("expected same hash")
	}
	if h.Hash("test1") == h.Hash("test2") {
		t.Error("expected different hashes")
	}
}

func TestXXHasher(t *testing.T) {
	h := XXHasher{}
	if h.Hash("test") == 0 {
		t.Error("expected non-zero hash")
	}
	if h.Hash(42) != h.Hash(42) {
		t.Error("expected same hash")
	}
	if h.Hash("test1") == h.Hash("test2") {
		t.Error("expected different hashes")
	}
}

func TestHashString(t *testing.T) {
	if HashString("test") == 0 {
		t.Error("expected non-zero hash")
	}
	if HashString("test") != HashString("test") {
		t.Error("expected same hash")
	}
}

func TestHashBytes(t *testing.T) {
	if HashBytes([]byte("test")) == 0 {
		t.Error("expected non-zero hash")
	}
	if HashBytes([]byte("test")) != HashBytes([]byte("test")) {
		t.Error("expected same hash")
	}
}

func TestXxHash32(t *testing.T) {
	if XxHash32("test") == 0 {
		t.Error("expected non-zero hash")
	}
	if XxHash32("test") != XxHash32("test") {
		t.Error("expected same hash")
	}
}

func BenchmarkFNVHasher(b *testing.B) {
	h := FNVHasher{}
	for i := 0; i < b.N; i++ {
		h.Hash("benchmark key")
	}
}

func BenchmarkXXHasher(b *testing.B) {
	h := XXHasher{}
	for i := 0; i < b.N; i++ {
		h.Hash("benchmark key")
	}
}
