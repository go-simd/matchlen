//go:build amd64

package matchlen

import (
	"math/rand"
	"testing"
)

func benchBulk(b *testing.B, fn func(a, bb []byte, limit int) int) {
	n := 4096
	a := make([]byte, n)
	rand.New(rand.NewSource(5)).Read(a)
	c := append([]byte(nil), a...)
	c[4000] ^= 1 // long common prefix, mismatch near the end
	b.SetBytes(4000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = fn(a, c, n)
	}
}

func BenchmarkBulkSSE(b *testing.B)  { benchBulk(b, bulkMatchSSE) }
func BenchmarkBulkAVX2(b *testing.B) { benchBulk(b, bulkMatchAVX2) }
