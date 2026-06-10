//go:build ignore

// Command gen produces bulkmatch_arm64.s with go-asmgen: the 16-byte NEON block
// loop for MatchLen. VEOR gives a^b (zero where equal); the first non-zero byte
// across the two 64-bit halves (via RBIT+CLZ) is the first difference. Run:
// go run bulkmatch_arm64_gen.go
package main

import (
	"fmt"
	"os"

	"github.com/go-asmgen/asmgen/abi"
	"github.com/go-asmgen/asmgen/arm64"
	"github.com/go-asmgen/asmgen/emit"
)

func main() {
	sig := abi.LayoutArgs(
		[]abi.Arg{abi.Slice("a"), abi.Slice("b"), abi.Scalar("limit", abi.Int64)},
		[]abi.Arg{abi.Scalar("ret", abi.Int64)},
	)
	b := arm64.NewFunc("bulkMatch", sig, 0)
	b.LoadArg("a_base", "R0").
		LoadArg("b_base", "R1").
		LoadArg("limit", "R2").
		Raw("MOVD $0, R3"). // i = 0
		Label("loop").
		Raw("ADD $16, R3, R4").
		Raw("CMP R2, R4").
		Raw("BGT done"). // i+16 > limit -> tail
		Raw("ADD R0, R3, R5").
		Raw("VLD1 (R5), [V0.B16]").
		Raw("ADD R1, R3, R6").
		Raw("VLD1 (R6), [V1.B16]").
		Raw("VEOR V1.B16, V0.B16, V2.B16"). // 0 where equal
		Raw("VMOV V2.D[0], R7").            // low 8 bytes
		Raw("CBNZ R7, lo").
		Raw("VMOV V2.D[1], R7"). // high 8 bytes
		Raw("CBNZ R7, hi").
		Raw("ADD $16, R3, R3").
		Raw("JMP loop").
		Label("lo").
		Raw("RBIT R7, R8").
		Raw("CLZ R8, R8"). // ctz(R7)
		Raw("LSR $3, R8, R8").
		Raw("ADD R3, R8, R8"). // i + byte index
		StoreRet("R8", "ret").
		Raw("RET").
		Label("hi").
		Raw("RBIT R7, R8").
		Raw("CLZ R8, R8").
		Raw("LSR $3, R8, R8").
		Raw("ADD $8, R8, R8"). // high half offset
		Raw("ADD R3, R8, R8").
		StoreRet("R8", "ret").
		Raw("RET").
		Label("done").
		StoreRet("R3", "ret").
		Ret()
	f := emit.NewFile("arm64")
	f.Add(b.Func())
	if err := os.WriteFile("bulkmatch_arm64.s", []byte(f.String()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("wrote bulkmatch_arm64.s")
}
