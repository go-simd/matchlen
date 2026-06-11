//go:build ignore

// Command gen produces bulkmatch_s390x.s with go-asmgen: the 16-byte vector
// block loop for MatchLen on z13+ (the vector facility is baseline there, so no
// runtime dispatch).
//
// Each iteration loads 16 bytes of a and b with VL and runs VFENEBS (Vector Find
// Element Not Equal, byte, with Condition Code) directly on the two vectors.
// VFENEBS scans the elements left-to-right and:
//   - if it finds an unequal element, it sets CC=1 and writes the byte index of
//     the first unequal element into byte 7 of the result vector;
//   - if all 16 bytes are equal, it sets CC=3.
// We loop while CC=3 (all equal) with BVS, and on a mismatch (CC=1, BLT-taken /
// fallthrough) extract the index with VLGVB $7.
//
// BIG-ENDIAN: s390x is the only big-endian target. VL puts the LOWEST memory
// address into the HIGH-order lane (element 0). VFENEBS scans from element 0
// upward, so the byte index it returns is already the offset of the first
// differing byte in memory order — no endian fix-up, no scan-direction reversal
// is needed (unlike the little-endian arches that count trailing zeros of a
// least-significant-byte-first word). The index is pinned by a position-dependent
// qemu test.
//
// Run: go run bulkmatch_s390x_gen.go
package main

import (
	"fmt"
	"os"

	"github.com/go-asmgen/asmgen/abi"
	"github.com/go-asmgen/asmgen/emit"
	"github.com/go-asmgen/asmgen/s390x"
)

func main() {
	sig := abi.LayoutArgs(
		[]abi.Arg{abi.Slice("a"), abi.Slice("b"), abi.Scalar("limit", abi.Int64)},
		[]abi.Arg{abi.Scalar("ret", abi.Int64)},
	)
	b := s390x.NewFunc("bulkMatch", sig, 0)
	b.LoadArg("a_base", "R1").
		LoadArg("b_base", "R2").
		LoadArg("limit", "R3").
		Raw("MOVD $0, R4"). // i = 0
		Label("loop").
		Raw("ADD $16, R4, R5").
		Raw("CMPBGT R5, R3, done"). // i+16 > limit -> tail
		Raw("ADD R1, R4, R6").
		Raw("VL (R6), V0"). // V0 = a[i:i+16]
		Raw("ADD R2, R4, R7").
		Raw("VL (R7), V1").          // V1 = b[i:i+16]
		Raw("VFENEBS V0, V1, V2").   // find first unequal byte; CC=1 found, CC=3 all-equal
		Raw("BVS next").            // CC=3 (all equal) -> advance
		Raw("VLGVB $7, V2, R8").    // R8 = byte index of first differing byte
		Raw("ADD R4, R8, R8").
		StoreRet("R8", "ret").
		Raw("RET").
		Label("next").
		Raw("ADD $16, R4, R4").
		Raw("BR loop").
		Label("done").
		StoreRet("R4", "ret").
		Ret()
	f := emit.NewFile("s390x")
	f.Add(b.Func())
	if err := os.WriteFile("bulkmatch_s390x.s", []byte(f.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("wrote bulkmatch_s390x.s")
}
