//go:build ignore

// Command gen produces bulkmatch_loong64.s with go-asmgen: the 16-byte LSX block
// loop for MatchLen. VXORV gives a^b (zero where equal); VMOVQ's element form
// reads each 64-bit half out to a GPR, and CTZV finds the first non-zero byte.
// Run: go run bulkmatch_loong64_gen.go
package main

import (
	"fmt"
	"os"

	"github.com/go-asmgen/asmgen/abi"
	"github.com/go-asmgen/asmgen/emit"
	"github.com/go-asmgen/asmgen/loong64"
)

func main() {
	sig := abi.LayoutArgs(
		[]abi.Arg{abi.Slice("a"), abi.Slice("b"), abi.Scalar("limit", abi.Int64)},
		[]abi.Arg{abi.Scalar("ret", abi.Int64)},
	)
	b := loong64.NewFunc("bulkMatch", sig, 0)
	b.LoadArg("a_base", "R4").
		LoadArg("b_base", "R5").
		LoadArg("limit", "R6").
		Raw("MOVV $0, R7"). // i = 0
		Label("loop").
		Raw("ADDV $16, R7, R8").
		Raw("BLT R6, R8, done"). // limit < i+16 -> tail
		Raw("ADDV R4, R7, R9").
		Raw("VMOVQ (R9), V0").
		Raw("ADDV R5, R7, R10").
		Raw("VMOVQ (R10), V1").
		Raw("VXORV V1, V0, V2").   // V2 = V0 ^ V1
		Raw("VMOVQ V2.V[0], R11"). // low 8 bytes -> GPR
		Raw("BNE R11, R0, lo").
		Raw("VMOVQ V2.V[1], R11"). // high 8 bytes -> GPR
		Raw("BNE R11, R0, hi").
		Raw("ADDV $16, R7, R7").
		Raw("JMP loop").
		Label("lo").
		Raw("CTZV R11, R12").
		Raw("SRLV $3, R12, R12").
		Raw("ADDV R7, R12, R12").
		StoreRet("R12", "ret").
		Raw("RET").
		Label("hi").
		Raw("CTZV R11, R12").
		Raw("SRLV $3, R12, R12").
		Raw("ADDV $8, R12, R12").
		Raw("ADDV R7, R12, R12").
		StoreRet("R12", "ret").
		Raw("RET").
		Label("done").
		StoreRet("R7", "ret").
		Ret()
	f := emit.NewFile("loong64")
	f.Add(b.Func())
	if err := os.WriteFile("bulkmatch_loong64.s", []byte(f.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("wrote bulkmatch_loong64.s")
}
