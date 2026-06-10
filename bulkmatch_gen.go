//go:build ignore

// Command gen produces bulkmatch_amd64.s with go-asmgen: the 16-byte SSE2 block
// loop for MatchLen. PCMPEQB builds a per-byte equal mask, PMOVMSKB moves it to a
// GPR; an all-ones mask (0xFFFF) means the whole block matched, otherwise NOTL+
// BSFL locate the first differing byte. Run: go run bulkmatch_gen.go
package main

import (
	"fmt"
	"os"

	"github.com/go-asmgen/asmgen/abi"
	"github.com/go-asmgen/asmgen/amd64"
	"github.com/go-asmgen/asmgen/emit"
)

func main() {
	sig := abi.LayoutArgs(
		[]abi.Arg{abi.Slice("a"), abi.Slice("b"), abi.Scalar("limit", abi.Int64)},
		[]abi.Arg{abi.Scalar("ret", abi.Int64)},
	)
	b := amd64.NewFunc("bulkMatch", sig, 0)
	b.LoadArg("a_base", "AX").
		LoadArg("b_base", "BX").
		LoadArg("limit", "CX").
		Raw("XORQ DI, DI"). // i = 0
		Label("loop").
		Raw("LEAQ 16(DI), R8").
		Raw("CMPQ R8, CX").
		Raw("JGT done"). // i+16 > limit -> tail
		Raw("MOVOU (AX)(DI*1), X0").
		Raw("MOVOU (BX)(DI*1), X1").
		Raw("PCMPEQB X1, X0").
		Raw("PMOVMSKB X0, R9").
		Raw("CMPL R9, $0xFFFF").
		Raw("JNE mism"). // some byte differs
		Raw("ADDQ $16, DI").
		Raw("JMP loop").
		Label("mism").
		Raw("NOTL R9").
		Raw("BSFL R9, R9"). // first differing byte (0..15)
		Raw("ADDQ DI, R9").
		StoreRet("R9", "ret").
		Raw("RET").
		Label("done").
		StoreRet("DI", "ret").
		Ret()
	f := emit.NewFile("amd64")
	f.Add(b.Func())
	if err := os.WriteFile("bulkmatch_amd64.s", []byte(f.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("wrote bulkmatch_amd64.s")
}
