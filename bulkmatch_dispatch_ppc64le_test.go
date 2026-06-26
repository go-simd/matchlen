//go:build ppc64le

package matchlen

import (
	"math/rand"
	"testing"

	"golang.org/x/sys/cpu"
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

// TestBulkMatchDispatchPPC64LE drives bulkMatch down both branches — the VSX
// kernel and the portable word-at-a-time fallback. The VSX kernel emits ISA-3.0
// (POWER9) instructions (CNTTZD) that raise SIGILL on POWER8, so the
// kernel-forcing branch runs only when the host is actually POWER9+ (mirroring
// the amd64 force test, which skips when the CPU lacks AVX2). The scalar-fallback
// branch is always exercised. The power9-targeted QEMU CI job and the native
// POWER9/POWER10 farm runs cover the kernel branch.
//
// bulkMatch only guarantees it stops on or before the first mismatch (the tail
// loop finishes the prefix), so each branch is validated by: (a) its result
// never overshoots the true prefix, and (b) running the scalar tail after it
// reproduces the true prefix exactly.
func TestBulkMatchDispatchPPC64LE(t *testing.T) {
	saved := hasVSX
	defer func() { hasVSX = saved }()

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

	// Scalar fallback: always safe on every ppc64le host (POWER8 included).
	hasVSX = false
	check("fallback")

	// VSX kernel: only force it on when the CPU is POWER9+, otherwise the CNTTZD
	// in bulkMatchVSX would SIGILL (e.g. on a POWER8 farm node).
	if !cpu.PPC64.IsPOWER9 {
		t.Log("CPU is pre-POWER9; VSX kernel branch not exercised on this host")
		return
	}
	hasVSX = true
	check("kernel")
}
