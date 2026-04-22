// disasm.go - Disassemble Go QuickJS regex bytecode
// Usage: go run disasm.go [pattern...]

package main

import (
	"encoding/binary"
	"fmt"
	"os"

	"github.com/akzj/go-quickjs/pkg/regexp"
)

// OpCode names matching libregexp-opcode.h
var opNames = map[int]string{
	0:  "invalid",
	1:  "char",
	2:  "char_i",
	3:  "char32",
	4:  "char32_i",
	5:  "dot",
	6:  "any",
	7:  "space",
	8:  "not_space",
	9:  "line_start",
	10: "line_start_m",
	11: "line_end",
	12: "line_end_m",
	13: "goto",
	14: "split_goto_first",
	15: "split_next_first",
	16: "match",
	17: "lookahead_match",
	18: "negative_lookahead_match",
	19: "save_start",
	20: "save_end",
	21: "save_reset",
	22: "loop",
	23: "loop_split_goto_first",
	24: "loop_split_next_first",
	25: "loop_check_adv_split_goto_first",
	26: "loop_check_adv_split_next_first",
	27: "set_i32",
	28: "word_boundary",
	29: "word_boundary_i",
	30: "not_word_boundary",
	31: "not_word_boundary_i",
	32: "back_reference",
	33: "back_reference_i",
	34: "backward_back_reference",
	35: "backward_back_reference_i",
	36: "range",
	37: "range_i",
	38: "range32",
	39: "range32_i",
	40: "lookahead",
	41: "negative_lookahead",
	42: "set_char_pos",
	43: "check_advance",
	44: "prev",
}

// Opcode sizes
var opcodeSize = []int{
	1, 3, 3, 5, 5, 1, 1, 1, 1, 1, 1, 1, 1, 5, 5, 5, 1, 1, 1, 2, 2, 3, 6, 10, 10, 10, 10, 6, 1, 1, 1, 1, 2, 2, 2, 2, 3, 3, 3, 3, 5, 5, 2, 2, 1,
}

func disasm(bc []byte) {
	fmt.Println("=== Disassembly ===")

	// Header
	flags := binary.LittleEndian.Uint16(bc[0:])
	captureCount := int(bc[2])
	registerCount := int(bc[3])
	bcLen := int(binary.LittleEndian.Uint32(bc[4:]))

	fmt.Printf("Header:\n")
	fmt.Printf("  flags: 0x%04x\n", flags)
	fmt.Printf("  capture_count: %d\n", captureCount)
	fmt.Printf("  register_count: %d\n", registerCount)
	fmt.Printf("  bytecode_len: %d\n", bcLen)

	// Bytecode starts after 8-byte header
	pos := 0
	bytecode := bc[8:]

	fmt.Printf("\nInstructions:\n")
	for pos < bcLen && pos < len(bytecode) {
		op := int(bytecode[pos])
		name := opNames[op]
		if name == "" {
			name = fmt.Sprintf("opcode_%d", op)
		}

		fmt.Printf("  [%3d] %-30s ", pos, name)

		// Print operands based on opcode
		switch op {
		case 1: // char
			val := binary.LittleEndian.Uint16(bytecode[pos+1:])
			fmt.Printf("'%c' (0x%04x)", rune(val), val)
			pos += 3
		case 2: // char_i
			val := binary.LittleEndian.Uint16(bytecode[pos+1:])
			fmt.Printf("0x%04x", val)
			pos += 3
		case 3: // char32
			val := binary.LittleEndian.Uint32(bytecode[pos+1:])
			fmt.Printf("0x%08x", val)
			pos += 5
		case 4: // char32_i
			val := binary.LittleEndian.Uint32(bytecode[pos+1:])
			fmt.Printf("0x%08x", val)
			pos += 5
		case 13: // goto
			offset := int32(binary.LittleEndian.Uint32(bytecode[pos+1:]))
			fmt.Printf("+%d", offset)
			pos += 5
		case 14, 15: // split_goto_first, split_next_first
			offset := int32(binary.LittleEndian.Uint32(bytecode[pos+1:]))
			fmt.Printf("+%d", offset)
			pos += 5
		case 19, 20: // save_start, save_end
			idx := int(bytecode[pos+1])
			fmt.Printf("reg=%d", idx)
			pos += 2
		case 21: // save_reset
			lo := int(bytecode[pos+1])
			hi := int(bytecode[pos+2])
			fmt.Printf("reg=%d..%d", lo, hi)
			pos += 3
		case 36, 37: // range, range_i
			n := int(binary.LittleEndian.Uint16(bytecode[pos+1:]))
			fmt.Printf("n=%d", n)
			pos += 3 + n*4
			continue
		case 38, 39: // range32, range32_i
			n := int(binary.LittleEndian.Uint16(bytecode[pos+1:]))
			fmt.Printf("n=%d", n)
			pos += 3 + n*8
			continue
		case 40, 41: // lookahead, negative_lookahead
			offset := int32(binary.LittleEndian.Uint32(bytecode[pos+1:]))
			fmt.Printf("+%d", offset)
			pos += 5
		default:
			if op < len(opcodeSize) {
				pos += opcodeSize[op]
			} else {
				pos++
			}
		}
		fmt.Println()
	}
}

func dumpHex(bc []byte) {
	fmt.Println("\nHex dump:")
	for i := 0; i < len(bc); i++ {
		fmt.Printf("%02x ", bc[i])
		if (i+1)%16 == 0 {
			fmt.Println()
		} else if (i+1)%8 == 0 {
			fmt.Print(" ")
		}
	}
	fmt.Println()
}

func main() {
	patterns := []string{"abc", "a+b", "a*b", "a?b", "test|best"}

	if len(os.Args) > 1 {
		patterns = os.Args[1:]
	}

	for _, pattern := range patterns {
		fmt.Printf("\n========================================\n")
		fmt.Printf("Pattern: %s\n", pattern)

		bc, err := regexp.Compile(pattern, 0, nil)
		if err != nil {
			fmt.Printf("Compile failed: %v\n", err)
			continue
		}

		fmt.Printf("Total length: %d bytes\n", len(bc))
		disasm(bc)
		dumpHex(bc)
	}
}
