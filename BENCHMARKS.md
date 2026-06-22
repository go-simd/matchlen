# Performance parity — go-simd/matchlen vs scalar baseline

**Methodology.** Apple M4 Max (arm64, NEON), macOS (Darwin 25.5.0), Go 1.26.4,
single core. `MatchLen` returns the length of the common prefix of two byte
slices — the LZ-family match-finder primitive. The fair reference is the naive
scalar byte-at-a-time compare loop (`refScalar`) every compressor uses without
SIMD; go-simd runs a NEON wide-compare kernel (go-asmgen) plus a scalar tail.
Inputs share a **full-length common prefix with the mismatch on the last byte**
(worst case — the loop runs the entire length), seed 2, sizes 16 B … 64 KiB;
`-benchtime=0.3s -count=3`, median reported. Correctness: `go test` (incl. fuzz)
matches `refScalar` on every input before benchmarking. Reproduce:

```
GOWORK=off go test -run='^$' -bench=Parity -benchmem -benchtime=0.3s -count=3 .
```

go-simd/matchlen has NEON on arm64, so these are real SIMD numbers.

| op | match len | go-simd (GB/s) | scalar baseline (GB/s) | speedup vs scalar | verdict |
|----|-----------|---------------:|-----------------------:|------------------:|---------|
| MatchLen | 16 B   |  5.71 | 3.07 |  1.86× | beats scalar (tail-dominated) |
| MatchLen | 64 B   | 18.86 | 3.45 |  5.47× | strong win |
| MatchLen | 256 B  | 34.39 | 2.87 | 11.98× | strong win |
| MatchLen | 4 KiB  | 32.48 | 3.49 |  9.31× | strong win |
| MatchLen | 64 KiB | 38.69 | 3.51 | 11.02× | **strong win** |

## Summary

* **Beats the scalar baseline 5.5–12×** for matches ≥ 64 B, sustaining
  ~33–39 GB/s vs the scalar loop's ~3.5 GB/s. Zero allocations.
* At 16 B the win is only 1.9× — the SIMD body covers one vector and the scalar
  tail dominates; expected and acceptable for sub-stride matches.
* There is no widely-used standalone Go reference library for this exact
  primitive (it lives inside each compressor, e.g. pierrec/lz4's internal
  `matchLen`); the scalar baseline is the correct fair reference.

### Action items
1. **amd64/AVX2 follow-up:** measure the AVX2 kernel on a real x86_64 VM (the
   memory note that go-simd's lz4 path beats pierrec/lz4 was an amd64 result;
   re-confirm here). Not measurable on this arm64 host.
2. Consider lowering the SIMD threshold / unrolling the 16–32 B range so short
   matches (very common in real LZ streams) get more of the SIMD win.
