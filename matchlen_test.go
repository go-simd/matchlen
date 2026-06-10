package matchlen

import (
	"bytes"
	"math/rand"
	"testing"
)

// ref is the trivial byte-by-byte common-prefix length.
func ref(a, b []byte) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

func TestMatchLen(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for trial := 0; trial < 5000; trial++ {
		n := rng.Intn(200)
		a := make([]byte, n)
		rng.Read(a)
		b := append([]byte(nil), a...)
		if len(b) > 0 && rng.Intn(2) == 0 { // inject a mismatch
			b[rng.Intn(len(b))] ^= 0xFF
		}
		if len(b) > 0 && rng.Intn(2) == 0 { // truncate
			b = b[:rng.Intn(len(b)+1)]
		}
		if got, want := MatchLen(a, b), ref(a, b); got != want {
			t.Fatalf("trial %d: MatchLen=%d want %d (la=%d lb=%d)", trial, got, want, len(a), len(b))
		}
	}
	for _, n := range []int{0, 1, 7, 8, 15, 16, 17, 31, 32, 33, 47, 48, 64, 255} {
		a := bytes.Repeat([]byte{'x'}, n)
		if got := MatchLen(a, a); got != n {
			t.Fatalf("identical n=%d: got %d", n, got)
		}
	}
}

func FuzzMatchLen(f *testing.F) {
	f.Add([]byte("hello world"), []byte("hello there"))
	f.Add([]byte{}, []byte{})
	f.Fuzz(func(t *testing.T, a, b []byte) {
		if got, want := MatchLen(a, b), ref(a, b); got != want {
			t.Fatalf("MatchLen=%d want %d", got, want)
		}
	})
}

func benchInputs() (a, c []byte) {
	buf := make([]byte, 4096)
	rand.New(rand.NewSource(2)).Read(buf)
	a = append([]byte(nil), buf...)
	c = append([]byte(nil), buf...)
	c[4000] ^= 1 // long shared prefix, mismatch near the end
	return
}

func BenchmarkMatchLen(b *testing.B) {
	a, c := benchInputs()
	b.SetBytes(4000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = MatchLen(a, c)
	}
}

func BenchmarkRef(b *testing.B) {
	a, c := benchInputs()
	b.SetBytes(4000)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ref(a, c)
	}
}
