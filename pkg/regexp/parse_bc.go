package regexp

import (
	"encoding/binary"
	"fmt"
)

// getU16 reads a little-endian uint16
func getU16(b []byte) uint16 {
	return binary.LittleEndian.Uint16(b)
}

// getU32 reads a little-endian uint32
func getU32(b []byte) uint32 {
	return binary.LittleEndian.Uint32(b)
}

// ParseBytecode dumps bytecode for debugging
func ParseBytecode(bc []byte) {
	fmt.Printf("=== Bytecode (%d bytes) ===\n", len(bc))

	if len(bc) < 8 {
		fmt.Printf("Too short\n")
		return
	}

	flags := getU16(bc[0:2])
	captureCount := bc[2]
	registerCount := bc[3]
	bcLen := getU32(bc[4:8])

	fmt.Printf("Header:\n")
	fmt.Printf("  flags: 0x%04x\n", flags)
	fmt.Printf("  captureCount: %d\n", captureCount)
	fmt.Printf("  registerCount: %d\n", registerCount)
	fmt.Printf("  bytecodeLen: %d\n", bcLen)

	fmt.Printf("\nBytecode (starting at offset 8):\n")

	pc := 8
	for pc < 8+int(bcLen) && pc < len(bc) {
		op := bc[pc]
		fmt.Printf("  [%2d] ", pc)

		switch OpCode(op) {
		case OpChar:
			if pc+3 <= len(bc) {
				ch := getU16(bc[pc+1 : pc+3])
				fmt.Printf("OpChar '%c' (0x%04x)", ch, ch)
			}
			pc += 3

		case OpSplitGotoFirst:
			if pc+5 <= len(bc) {
				offset := int32(getU32(bc[pc+1 : pc+5]))
				fmt.Printf("OpSplitGotoFirst offset=%d (target=%d)", offset, pc+5+int(offset))
			}
			pc += 5

		case OpGoto:
			if pc+5 <= len(bc) {
				offset := int32(getU32(bc[pc+1 : pc+5]))
				fmt.Printf("OpGoto offset=%d (target=%d)", offset, pc+5+int(offset))
			}
			pc += 5

		case OpRange:
			if pc+3 <= len(bc) {
				n := getU16(bc[pc+1 : pc+3])
				fmt.Printf("OpRange n=%d", n)
				for i := 0; i < int(n); i++ {
					low := getU16(bc[pc+3+i*4 : pc+5+i*4])
					high := getU16(bc[pc+5+i*4 : pc+7+i*4])
					fmt.Printf(" [%c-%c]", low, high)
				}
				pc += 3 + int(n)*4
				fmt.Println()
				continue
			}
			pc += 3

		case OpSaveStart:
			fmt.Printf("OpSaveStart idx=%d", bc[pc+1])
			pc += 2

		case OpSaveEnd:
			fmt.Printf("OpSaveEnd idx=%d", bc[pc+1])
			pc += 2

		case OpMatch:
			fmt.Printf("OpMatch")
			pc++

		case OpBackReference:
			fmt.Printf("OpBackReference idx=%d", bc[pc+1])
			pc += 2

		case OpBackReferenceI:
			fmt.Printf("OpBackReferenceI idx=%d", bc[pc+1])
			pc += 2

		default:
			fmt.Printf("OP_%d (0x%02x)", op, op)
			pc++
		}
		fmt.Println()
	}
}