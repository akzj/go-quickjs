package main

import (
	"encoding/binary"
	"fmt"
	"github.com/akzj/go-quickjs/pkg/regexp"
)

func main() {
	patterns := []string{
		`\d+`,
		`a+`,
		`a*`,
		`a?`,
	}

	for _, p := range patterns {
		fmt.Printf("\n=== Pattern: %s ===\n", p)
		bc, err := regexp.Compile(p, 0, nil)
		if err != nil {
			fmt.Printf("Compile error: %v\n", err)
			continue
		}
		dumpBytecode(bc)
	}
}

func dumpBytecode(bc []byte) {
	fmt.Printf("Total length: %d bytes\n", len(bc))
	
	// Header
	headerLen := regexp.HeaderLen
	fmt.Printf("Header (%d bytes):\n", headerLen)
	fmt.Printf("  Flags: 0x%02x\n", bc[0])
	fmt.Printf("  CharSize: %d\n", bc[1])
	fmt.Printf("  CaptureCount: %d\n", bc[2])
	fmt.Printf("  StackSize: %d\n", bc[3])
	
	pos := headerLen
	for pos < len(bc) {
		op := regexp.OpCode(bc[pos])
		if op == 0 {
			break
		}
		fmt.Printf("  [%3d] ", pos)
		switch op {
		case regexp.OpChar:
			ch := binary.LittleEndian.Uint16(bc[pos+1:])
			fmt.Printf("OpChar '%c' (0x%02x)\n", ch, ch)
			pos += 3
		case regexp.OpCharI:
			ch := binary.LittleEndian.Uint16(bc[pos+1:])
			fmt.Printf("OpCharI '%c' (0x%02x)\n", ch, ch)
			pos += 3
		case regexp.OpChar32:
			ch := binary.LittleEndian.Uint32(bc[pos+1:])
			fmt.Printf("OpChar32 0x%08x\n", ch)
			pos += 5
		case regexp.OpChar32I:
			ch := binary.LittleEndian.Uint32(bc[pos+1:])
			fmt.Printf("OpChar32I 0x%08x\n", ch)
			pos += 5
		case regexp.OpRange:
			n := int(binary.LittleEndian.Uint16(bc[pos+1:]))
			fmt.Printf("OpRange n=%d\n", n)
			pos += 3 + n*4
		case regexp.OpRangeI:
			n := int(binary.LittleEndian.Uint16(bc[pos+1:]))
			fmt.Printf("OpRangeI n=%d\n", n)
			pos += 3 + n*4
		case regexp.OpRange32:
			n := int(binary.LittleEndian.Uint16(bc[pos+1:]))
			fmt.Printf("OpRange32 n=%d\n", n)
			pos += 3 + n*8
		case regexp.OpRange32I:
			n := int(binary.LittleEndian.Uint16(bc[pos+1:]))
			fmt.Printf("OpRange32I n=%d\n", n)
			pos += 3 + n*8
		case regexp.OpDot:
			fmt.Printf("OpDot\n")
			pos++
		case regexp.OpAny:
			fmt.Printf("OpAny\n")
			pos++
		case regexp.OpSpace:
			fmt.Printf("OpSpace\n")
			pos++
		case regexp.OpNotSpace:
			fmt.Printf("OpNotSpace\n")
			pos++
		case regexp.OpGoto:
			offset := binary.LittleEndian.Uint32(bc[pos+1:])
			signed := int32(offset)
			target := pos + 5 + int(signed)
			fmt.Printf("OpGoto offset=%d (target=%d)\n", signed, target)
			pos += 5
		case regexp.OpSplitGotoFirst:
			offset := binary.LittleEndian.Uint32(bc[pos+1:])
			signed := int32(offset)
			target := pos + 5 + int(signed)
			fmt.Printf("OpSplitGotoFirst offset=%d (target=%d)\n", signed, target)
			pos += 5
		case regexp.OpSplitNextFirst:
			offset := binary.LittleEndian.Uint32(bc[pos+1:])
			signed := int32(offset)
			target := pos + 5 + int(signed)
			fmt.Printf("OpSplitNextFirst offset=%d (target=%d)\n", signed, target)
			pos += 5
		case regexp.OpMatch:
			fmt.Printf("OpMatch\n")
			pos++
		case regexp.OpLoop:
			idx := bc[pos+1]
			offset := binary.LittleEndian.Uint32(bc[pos+2:])
			signed := int32(offset)
			target := pos + 6 + int(signed)
			fmt.Printf("OpLoop idx=%d offset=%d (target=%d)\n", idx, signed, target)
			pos += 6
		case regexp.OpLoopSplitGotoFirst:
			idx := bc[pos+1]
			offset := binary.LittleEndian.Uint32(bc[pos+2:])
			signed := int32(offset)
			target := pos + 6 + int(signed)
			fmt.Printf("OpLoopSplitGotoFirst idx=%d offset=%d (target=%d)\n", idx, signed, target)
			pos += 10
		case regexp.OpLoopSplitNextFirst:
			idx := bc[pos+1]
			offset := binary.LittleEndian.Uint32(bc[pos+2:])
			signed := int32(offset)
			target := pos + 6 + int(signed)
			fmt.Printf("OpLoopSplitNextFirst idx=%d offset=%d (target=%d)\n", idx, signed, target)
			pos += 10
		case regexp.OpSaveStart:
			idx := bc[pos+1]
			fmt.Printf("OpSaveStart [%d]\n", idx)
			pos += 2
		case regexp.OpSaveEnd:
			idx := bc[pos+1]
			fmt.Printf("OpSaveEnd [%d]\n", idx)
			pos += 2
		case regexp.OpSaveReset:
			idx1 := bc[pos+1]
			idx2 := bc[pos+2]
			fmt.Printf("OpSaveReset [%d-%d]\n", idx1, idx2)
			pos += 3
		case regexp.OpLookahead:
			offset := binary.LittleEndian.Uint32(bc[pos+1:])
			signed := int32(offset)
			target := pos + 5 + int(signed)
			fmt.Printf("OpLookahead offset=%d (target=%d)\n", signed, target)
			pos += 5
		case regexp.OpNegativeLookahead:
			offset := binary.LittleEndian.Uint32(bc[pos+1:])
			signed := int32(offset)
			target := pos + 5 + int(signed)
			fmt.Printf("OpNegativeLookahead offset=%d (target=%d)\n", signed, target)
			pos += 5
		case regexp.OpWordBoundary:
			fmt.Printf("OpWordBoundary\n")
			pos++
		case regexp.OpNotWordBoundary:
			fmt.Printf("OpNotWordBoundary\n")
			pos++
		case regexp.OpLineStart:
			fmt.Printf("OpLineStart\n")
			pos++
		case regexp.OpLineEnd:
			fmt.Printf("OpLineEnd\n")
			pos++
		case regexp.OpBackReference:
			idx := bc[pos+1]
			fmt.Printf("OpBackReference [%d]\n", idx)
			pos += 2
		case regexp.OpPrev:
			fmt.Printf("OpPrev\n")
			pos++
		default:
			fmt.Printf("UNKNOWN opcode %d\n", op)
			pos++
		}
		if pos > len(bc) {
			fmt.Printf("  ERROR: past end of bytecode\n")
			break
		}
	}
}
