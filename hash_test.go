package mappo

import (
	"testing"
)

func TestStringHasher(t *testing.T) {
	h := StringHasher{}
	if h.Hash("test") == 0 {
		t.Error("expected non-zero hash")
	}
	if h.Hash("test") != h.Hash("test") {
		t.Error("expected same hash")
	}
	if h.Hash("test1") == h.Hash("test2") {
		t.Error("expected different hashes")
	}
}

func TestIntHasher(t *testing.T) {
	h := IntHasher{}
	if h.Hash(42) == 0 {
		t.Error("expected non-zero hash")
	}
	if h.Hash(42) != h.Hash(42) {
		t.Error("expected same hash")
	}
	if h.Hash(1) == h.Hash(2) {
		t.Error("expected different hashes")
	}
}

func TestInt64Hasher(t *testing.T) {
	h := Int64Hasher{}
	if h.Hash(42) == 0 {
		t.Error("expected non-zero hash")
	}
	if h.Hash(42) != h.Hash(42) {
		t.Error("expected same hash")
	}
	if h.Hash(1) == h.Hash(2) {
		t.Error("expected different hashes")
	}
}

func TestBytesHasher(t *testing.T) {
	h := BytesHasher{}
	if h.Hash([]byte("test")) == 0 {
		t.Error("expected non-zero hash")
	}
	if h.Hash([]byte("test")) != h.Hash([]byte("test")) {
		t.Error("expected same hash")
	}
	if h.Hash([]byte("test1")) == h.Hash([]byte("test2")) {
		t.Error("expected different hashes")
	}
}

func TestMaphashHasherString(t *testing.T) {
	h := NewMaphashHasher[string]()
	if h.Hash("test") == 0 {
		t.Error("expected non-zero hash")
	}
	if h.Hash("test") != h.Hash("test") {
		t.Error("expected same hash")
	}
	if h.Hash("test1") == h.Hash("test2") {
		t.Error("expected different hashes")
	}
}

func TestMaphashHasherInt(t *testing.T) {
	h := NewMaphashHasher[int]()
	if h.Hash(42) == 0 {
		t.Error("expected non-zero hash")
	}
	if h.Hash(42) != h.Hash(42) {
		t.Error("expected same hash")
	}
	if h.Hash(1) == h.Hash(2) {
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
	if HashString("test1") == HashString("test2") {
		t.Error("expected different hashes")
	}
}

func TestHashBytes(t *testing.T) {
	if HashBytes([]byte("test")) == 0 {
		t.Error("expected non-zero hash")
	}
	if HashBytes([]byte("test")) != HashBytes([]byte("test")) {
		t.Error("expected same hash")
	}
	if HashBytes([]byte("test1")) == HashBytes([]byte("test2")) {
		t.Error("expected different hashes")
	}
}

func TestXxHash64(t *testing.T) {
	if XxHash64("test") == 0 {
		t.Error("expected non-zero hash")
	}
	if XxHash64("test") != XxHash64("test") {
		t.Error("expected same hash")
	}
	if XxHash64("test1") == XxHash64("test2") {
		t.Error("expected different hashes")
	}
}

func TestXxHash32(t *testing.T) {
	if XxHash32("test") == 0 {
		t.Error("expected non-zero hash")
	}
	if XxHash32("test") != XxHash32("test") {
		t.Error("expected same hash")
	}
	if XxHash32("test1") == XxHash32("test2") {
		t.Error("expected different hashes")
	}
}

func TestXxHash64Bytes(t *testing.T) {
	if XxHash64Bytes([]byte("test")) == 0 {
		t.Error("expected non-zero hash")
	}
	if XxHash64Bytes([]byte("test")) != XxHash64Bytes([]byte("test")) {
		t.Error("expected same hash")
	}
	if XxHash64Bytes([]byte("test1")) == XxHash64Bytes([]byte("test2")) {
		t.Error("expected different hashes")
	}
}

func BenchmarkStringHasher(b *testing.B) {
	h := StringHasher{}
	for i := 0; i < b.N; i++ {
		h.Hash("benchmark key")
	}
}

func BenchmarkIntHasher(b *testing.B) {
	h := IntHasher{}
	for i := 0; i < b.N; i++ {
		h.Hash(42)
	}
}

func BenchmarkInt64Hasher(b *testing.B) {
	h := Int64Hasher{}
	for i := 0; i < b.N; i++ {
		h.Hash(42)
	}
}

func BenchmarkBytesHasher(b *testing.B) {
	h := BytesHasher{}
	for i := 0; i < b.N; i++ {
		h.Hash([]byte("benchmark key"))
	}
}

func BenchmarkMaphashHasherString(b *testing.B) {
	h := NewMaphashHasher[string]()
	for i := 0; i < b.N; i++ {
		h.Hash("benchmark key")
	}
}

func BenchmarkMaphashHasherInt(b *testing.B) {
	h := NewMaphashHasher[int]()
	for i := 0; i < b.N; i++ {
		h.Hash(42)
	}
}

func BenchmarkHashString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		HashString("benchmark key")
	}
}

func BenchmarkHashBytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		HashBytes([]byte("benchmark key"))
	}
}

func BenchmarkXxHash64(b *testing.B) {
	for i := 0; i < b.N; i++ {
		XxHash64("benchmark key")
	}
}

func BenchmarkXxHash32(b *testing.B) {
	for i := 0; i < b.N; i++ {
		XxHash32("benchmark key")
	}
}

func BenchmarkXxHash64Bytes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		XxHash64Bytes([]byte("benchmark key"))
	}
}
