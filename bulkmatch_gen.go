//go:build ignore

// Command gen produces bulkmatch_amd64.s with go-asmgen: a 16-byte SSE2 block
// loop (bulkMatchSSE) and a 32-byte AVX2 loop (bulkMatchAVX2) for MatchLen.
// PCMPEQB/VPCMPEQB build a per-byte equal mask, P/VPMOVMSKB move it to a GPR; an
// all-ones mask means the whole block matched, else NOT+BSF locate the first diff.
// Run: go run bulkmatch_gen.go
package main

import (
	"fmt"
	"os"

	"github.com/go-asmgen/asmgen/abi"
	"github.com/go-asmgen/asmgen/amd64"
	"github.com/go-asmgen/asmgen/emit"
)

func sig() abi.Signature {
	return abi.LayoutArgs(
		[]abi.Arg{abi.Slice("a"), abi.Slice("b"), abi.Scalar("limit", abi.Int64)},
		[]abi.Arg{abi.Scalar("ret", abi.Int64)},
	)
}

func main() {
	f := emit.NewFile("amd64")

	// SSE2: 16-byte stride.
	s := amd64.NewFunc("bulkMatchSSE", sig(), 0)
	s.LoadArg("a_base", "AX").LoadArg("b_base", "BX").LoadArg("limit", "CX").
		Raw("XORQ DI, DI").
		Label("sloop").
		Raw("LEAQ 16(DI), R8").Raw("CMPQ R8, CX").Raw("JGT sdone").
		Raw("MOVOU (AX)(DI*1), X0").Raw("MOVOU (BX)(DI*1), X1").
		Raw("PCMPEQB X1, X0").Raw("PMOVMSKB X0, R9").
		Raw("CMPL R9, $0xFFFF").Raw("JNE smism").
		Raw("ADDQ $16, DI").Raw("JMP sloop").
		Label("smism").Raw("NOTL R9").Raw("BSFL R9, R9").Raw("ADDQ DI, R9").
		StoreRet("R9", "ret").Raw("RET").
		Label("sdone").StoreRet("DI", "ret").Ret()
	f.Add(s.Func())

	// AVX2: 32-byte stride; VZEROUPPER before returning to avoid the AVX/SSE
	// transition penalty.
	v := amd64.NewFunc("bulkMatchAVX2", sig(), 0)
	v.LoadArg("a_base", "AX").LoadArg("b_base", "BX").LoadArg("limit", "CX").
		Raw("XORQ DI, DI").
		Label("vloop").
		Raw("LEAQ 32(DI), R8").Raw("CMPQ R8, CX").Raw("JGT vdone").
		Raw("VMOVDQU (AX)(DI*1), Y0").Raw("VMOVDQU (BX)(DI*1), Y1").
		Raw("VPCMPEQB Y1, Y0, Y0").Raw("VPMOVMSKB Y0, R9").
		Raw("CMPL R9, $-1").Raw("JNE vmism").
		Raw("ADDQ $32, DI").Raw("JMP vloop").
		Label("vmism").Raw("NOTL R9").Raw("BSFL R9, R9").Raw("ADDQ DI, R9").
		Raw("VZEROUPPER").StoreRet("R9", "ret").Raw("RET").
		Label("vdone").Raw("VZEROUPPER").StoreRet("DI", "ret").Ret()
	f.Add(v.Func())

	if err := os.WriteFile("bulkmatch_amd64.s", []byte(f.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("wrote bulkmatch_amd64.s")
}
