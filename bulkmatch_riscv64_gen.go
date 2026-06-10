//go:build ignore

// Command gen produces bulkmatch_riscv64.s with go-asmgen: the 16-byte RVV block
// loop for MatchLen. VXOR gives a^b (zero where equal); VMSNE builds a mask of
// the non-zero (differing) bytes and vfirst.m returns the first one's index
// (-1 if none). Run: go run bulkmatch_riscv64_gen.go
package main

import (
	"fmt"
	"os"

	"github.com/go-asmgen/asmgen/abi"
	"github.com/go-asmgen/asmgen/emit"
	"github.com/go-asmgen/asmgen/riscv64"
)

func main() {
	sig := abi.LayoutArgs(
		[]abi.Arg{abi.Slice("a"), abi.Slice("b"), abi.Scalar("limit", abi.Int64)},
		[]abi.Arg{abi.Scalar("ret", abi.Int64)},
	)
	b := riscv64.NewFunc("bulkMatch", sig, 0)
	b.LoadArg("a_base", "X5").
		LoadArg("b_base", "X6").
		LoadArg("limit", "X7").
		Raw("VSETVLI $16, E8, M1, TA, MA, X8"). // VL = 16 bytes
		Raw("MOV $0, X9").                      // i = 0
		Label("loop").
		Raw("ADD $16, X9, X10").
		Raw("BLT X7, X10, done"). // limit < i+16 -> tail
		Raw("ADD X5, X9, X11").
		Raw("VLE8V (X11), V1").
		Raw("ADD X6, X9, X12").
		Raw("VLE8V (X12), V2").
		Raw("VXORVV V1, V2, V3").  // V3 = V2 ^ V1
		Raw("VMSNEVI $0, V3, V0"). // V0 = mask(V3 != 0)
		Raw("VFIRSTM V0, X13").    // first differing index, or -1
		Raw("BGE X13, X0, found").
		Raw("ADD $16, X9, X9").
		Raw("JMP loop").
		Label("found").
		Raw("ADD X9, X13, X13").
		StoreRet("X13", "ret").
		Raw("RET").
		Label("done").
		StoreRet("X9", "ret").
		Ret()
	f := emit.NewFile("riscv64")
	f.Add(b.Func())
	if err := os.WriteFile("bulkmatch_riscv64.s", []byte(f.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("wrote bulkmatch_riscv64.s")
}
