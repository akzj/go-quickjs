package regexp

import (
	"fmt"
	"testing"

	"github.com/akzj/go-quickjs/internal/cutils"
)

func TestDebugBytecodeLayout(t *testing.T) {
	bc, err := Compile("a*b", 0, nil)
	if err != nil {
		t.Fatalf("Compile error: %v", err)
	}
	
	fmt.Printf("=== Bytecode Analysis for 'a*b' ===\n\n")
	
	// Print header
	fmt.Printf("Header:\n")
	fmt.Printf("  Flags: 0x%02x%02x\n", bc[1], bc[0])
	fmt.Printf("  Capture count: %d\n", bc[2])
	fmt.Printf("  Register count: %d\n", bc[3])
	fmt.Printf("  Bytecode length: %d\n", cutils.GetU32(bc[4:8]))
	
	fmt.Printf("\nBytecode bytes (absolute positions):\n")
	for i := 8; i < len(bc); i++ {
		fmt.Printf("  [%2d] = 0x%02x\n", i, bc[i])
	}
	
	// Manually trace through execution
	fmt.Printf("\n=== Execution Trace ===\n")
	
	// Start: PC = 8
	pc := 8
	for pc < len(bc) && pc >= 8 {
		op := bc[pc]
		fmt.Printf("\nPC=%d: opcode=0x%02x ", pc, op)
		
		switch op {
		case 0x0e: // split_goto_first
			offset := int32(cutils.GetU32(bc[pc+1:pc+5]))
			target := pc + 5 + int(offset)
			fmt.Printf("(split_goto_first) offset=%d target=%d", offset, target)
			fmt.Printf("\n  -> Will push PC=%d, jump to PC=%d", pc+5, target)
			pc = target
			
		case 0x0d: // goto
			offset := int32(cutils.GetU32(bc[pc+1:pc+5]))
			target := pc + 5 + int(offset)
			fmt.Printf("(goto) offset=%d target=%d", offset, target)
			pc = target
			
		case 0x06: // any
			fmt.Printf("(any) - consumes one char")
			pc++
			
		case 0x13: // save_start
			idx := bc[pc+1]
			fmt.Printf("(save_start) idx=%d", idx)
			pc += 2
			
		case 0x14: // save_end
			idx := bc[pc+1]
			fmt.Printf("(save_end) idx=%d", idx)
			pc += 2
			
		case 0x01: // char
			val := cutils.GetU16(bc[pc+1:pc+3])
			fmt.Printf("(char) '%c'", rune(val))
			pc += 3
			
		case 0x10: // match
			fmt.Printf("(MATCH!)")
			break
			
		default:
			fmt.Printf("(UNKNOWN!)")
			break
		}
		
		if pc == 40 {
			fmt.Printf("\n\nPC would be 40 (past end of bytecode)")
			break
		}
	}
	
	fmt.Printf("\n\n=== Check what opcodes mean ===\n")
	fmt.Printf("OpSplitGotoFirst = 0x%02x\n", OpSplitGotoFirst)
	fmt.Printf("OpGoto = 0x%02x\n", OpGoto)
	fmt.Printf("OpAny = 0x%02x\n", OpAny)
	fmt.Printf("OpSaveStart = 0x%02x\n", OpSaveStart)
	fmt.Printf("OpSaveEnd = 0x%02x\n", OpSaveEnd)
	fmt.Printf("OpChar = 0x%02x\n", OpChar)
	fmt.Printf("OpMatch = 0x%02x\n", OpMatch)
}
