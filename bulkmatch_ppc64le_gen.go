//go:build ignore

// Command gen produces bulkmatch_ppc64le.s with go-asmgen: the 16-byte VSX block
// loop for MatchLen on POWER8+ (VSX is baseline, so no runtime dispatch).
//
// Each iteration loads 16 bytes of a and b with LXVD2X, compares them byte-wise
// with VCMPEQUB (0xFF where equal, 0x00 where differing), then locates the first
// differing byte. The two doublewords of the compare result are moved to GPRs
// with MFVSRD and scanned with CNTTZD (count trailing zeros), which finds the
// lowest-order byte — i.e. the lowest memory address.
//
// VSX↔AltiVec aliasing: an AltiVec register Vn is the SAME physical register as
// VSX register VS(32+n) (NOT VSn). LXVD2X writes a VS register, so we load into
// VS32/VS33 and then refer to those same registers as V0/V1 for the AltiVec
// VCMPEQUB. MFVSRD reads VS34 (= V2), the compare result.
//
// Endianness: ppc64le is little-endian. LXVD2X loads two doublewords; the FIRST
// doubleword (the one MFVSRD reads directly) holds memory bytes 0..7 and the
// second holds bytes 8..15. Within each doubleword the lowest memory address is
// the least-significant byte, so CNTTZD (count trailing zeros) on the
// per-byte-differs mask finds the lowest-address mismatch and >>3 turns the bit
// index into a byte offset. The high half (bytes 8..15) is brought into a GPR
// with VSLDOI $8 then MFVSRD. This exact mapping (which doubleword is which, and
// the LE within-doubleword order) is pinned by a position-dependent qemu test.
//
// Run: go run bulkmatch_ppc64le_gen.go
package main

import (
	"fmt"
	"os"

	"github.com/go-asmgen/asmgen/abi"
	"github.com/go-asmgen/asmgen/emit"
	"github.com/go-asmgen/asmgen/ppc64"
)

func main() {
	sig := abi.LayoutArgs(
		[]abi.Arg{abi.Slice("a"), abi.Slice("b"), abi.Scalar("limit", abi.Int64)},
		[]abi.Arg{abi.Scalar("ret", abi.Int64)},
	)
	b := ppc64.NewFunc("bulkMatch", sig, 0)
	b.LoadArg("a_base", "R3").
		LoadArg("b_base", "R4").
		LoadArg("limit", "R5").
		Raw("MOVD $0, R6"). // i = 0
		Label("loop").
		Raw("ADD $16, R6, R7").
		Raw("CMP R5, R7").
		Raw("BLT done"). // limit < i+16 -> tail
		Raw("ADD R3, R6, R8").
		Raw("LXVD2X (R8), VS32"). // V0 = a[i:i+16]
		Raw("ADD R4, R6, R9").
		Raw("LXVD2X (R9), VS33"). // V1 = b[i:i+16]
		Raw("VCMPEQUB V0, V1, V2"). // V2 byte = 0xFF if equal else 0x00
		// First doubleword (no shift) = memory bytes 0..7.
		Raw("MFVSRD VS34, R10").  // R10 = compare bytes of the low-address 8
		Raw("NOR R10, R10, R11"). // R11 = ~R10: 0xFF where DIFFERING
		Raw("CMP R11, $0").
		Raw("BNE lo").
		// Second doubleword = memory bytes 8..15.
		Raw("VSLDOI $8, V2, V2, V3"). // V3 high dword := V2 low (8..15) dword
		Raw("MFVSRD VS35, R10").      // R10 = compare bytes of the high-address 8
		Raw("NOR R10, R10, R11").
		Raw("CMP R11, $0").
		Raw("BNE hi").
		Raw("ADD $16, R6, R6").
		Raw("BR loop").
		Label("lo").
		Raw("CNTTZD R11, R12").
		Raw("SRD $3, R12, R12").
		Raw("ADD R6, R12, R12").
		StoreRet("R12", "ret").
		Raw("RET").
		Label("hi").
		Raw("CNTTZD R11, R12").
		Raw("SRD $3, R12, R12").
		Raw("ADD $8, R12, R12").
		Raw("ADD R6, R12, R12").
		StoreRet("R12", "ret").
		Raw("RET").
		Label("done").
		StoreRet("R6", "ret").
		Ret()
	f := emit.NewFile("ppc64le")
	f.Add(b.Func())
	if err := os.WriteFile("bulkmatch_ppc64le.s", []byte(f.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("wrote bulkmatch_ppc64le.s")
}
