//go:build amd64

package matchlen

import (
	"math/rand"
	"testing"
)

// truePrefix is the byte-by-byte common-prefix length up to limit.
func truePrefix(a, b []byte, limit int) int {
	n := 0
	for n < limit && a[n] == b[n] {
		n++
	}
	return n
}

// finish replays MatchLen's scalar tail: bulkMatch may stop on a stride boundary
// short of the true prefix, leaving the remainder to the caller.
func finish(a, b []byte, limit, n int) int {
	for n < limit && a[n] == b[n] {
		n++
	}
	return n
}

// TestBulkMatchDispatch drives the bulkMatch dispatcher down both of its amd64
// branches — the AVX2 path and the SSE fallback — regardless of which one the
// host CPU would pick. The SSE branch (SSE2 baseline) always runs; the AVX2
// branch runs only when the CPU actually has AVX2 (forcing it on a non-AVX2 box
// would #UD). The native amd64 CI runner has AVX2, so both branches are covered
// there, making it the authoritative gate.
//
// bulkMatch only guarantees it stops on or before the first mismatch (the tail
// loop finishes the prefix), so each branch is validated by: (a) its kernel
// result never overshoots the true prefix, and (b) running the scalar tail after
// it reproduces the true prefix exactly.
func TestBulkMatchDispatch(t *testing.T) {
	saved := hasAVX2
	defer func() { hasAVX2 = saved }()

	rng := rand.New(rand.NewSource(99))
	check := func(label string) {
		for trial := 0; trial < 600; trial++ {
			n := rng.Intn(300)
			a := make([]byte, n)
			rng.Read(a)
			b := append([]byte(nil), a...)
			if n > 0 && rng.Intn(2) == 0 {
				b[rng.Intn(n)] ^= 0xFF // inject a mismatch
			}
			limit := len(a)
			if len(b) < limit {
				limit = len(b)
			}
			want := truePrefix(a, b, limit)
			got := bulkMatch(a, b, limit)
			if got > want {
				t.Fatalf("%s trial %d: bulkMatch=%d overshoots prefix %d", label, trial, got, want)
			}
			if full := finish(a, b, limit, got); full != want {
				t.Fatalf("%s trial %d: bulkMatch+tail=%d want %d", label, trial, full, want)
			}
		}
	}

	// SSE branch: SSE2 is baseline, always safe.
	hasAVX2 = false
	check("sse")

	// AVX2 branch: only when the CPU has it.
	if saved {
		hasAVX2 = true
		check("avx2")
	} else {
		t.Log("CPU lacks AVX2; AVX2 dispatch branch not exercised on this host")
	}
}
