package matchlen

// Standardized performance-parity harness: go-simd/matchlen (SIMD dispatch) vs
// the portable scalar byte-at-a-time baseline (refScalar). MatchLen is the LZ
// match-finder primitive, so the fair reference is the naive scalar compare
// loop every compressor would otherwise use. Run:
//
//	GOWORK=off go test -run=^$ -bench='Parity' -benchmem .
//
// Inputs share a long common prefix with a mismatch near the end, modelling a
// real match-extension probe (the worst case for the loop — it runs the full
// length). b.SetBytes(matched length) so `go test` reports MB/s.

import (
	"math/rand"
	"testing"
)

func refScalar(a, b []byte) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

var parityLens = []int{16, 64, 256, 4096, 65536}

func parityPair(n int) (a, c []byte) {
	buf := make([]byte, n)
	rand.New(rand.NewSource(2)).Read(buf)
	a = append([]byte(nil), buf...)
	c = append([]byte(nil), buf...)
	c[n-1] ^= 1 // full-length shared prefix, mismatch on the last byte
	return
}

func lenLabel(n int) string {
	switch n {
	case 16:
		return "16B"
	case 64:
		return "64B"
	case 256:
		return "256B"
	case 4096:
		return "4KiB"
	case 65536:
		return "64KiB"
	}
	return "?"
}

func BenchmarkParity(b *testing.B) {
	for _, n := range parityLens {
		a, c := parityPair(n)
		b.Run(lenLabel(n)+"/gosimd", func(b *testing.B) {
			b.SetBytes(int64(n))
			for i := 0; i < b.N; i++ {
				_ = MatchLen(a, c)
			}
		})
		b.Run(lenLabel(n)+"/scalar", func(b *testing.B) {
			b.SetBytes(int64(n))
			for i := 0; i < b.N; i++ {
				_ = refScalar(a, c)
			}
		})
	}
}
