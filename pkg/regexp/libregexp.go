// Package regexp implements a regular expression engine compatible with JavaScript's RegExp.
//
// This is a port of QuickJS's libregexp.c to Go, maintaining exact behavioral compatibility.
package regexp

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/akzj/go-quickjs/internal/cutils"
	quickunicode "github.com/akzj/go-quickjs/pkg/unicode"
)

// ============================================================================
// Constants and Flags
// ============================================================================

// Flags for regex compilation (matching JavaScript RegExp flags)
const (
	FlagGlobal      = 1 << 0
	FlagIgnoreCase  = 1 << 1
	FlagMultiline   = 1 << 2
	FlagDotAll      = 1 << 3
	FlagUnicode     = 1 << 4
	FlagSticky      = 1 << 5
	FlagIndices     = 1 << 6 // Unused by libregexp, just recorded.
	FlagNamedGroups = 1 << 7 // Named groups are present in the regexp
	FlagUnicodeSets = 1 << 8
)

// Return codes for lre_exec
const (
	RetMemoryError = -1
	RetTimeout     = -2
	RetNoMatch     = 0
	RetMatch       = 1
)

// Group name trailer length including trailing '\0'
const GroupNameTrailerLen = 2

// Limits
const (
	CaptureCountMax      = 255
	RegisterCountMax    = 255
	InterruptCounterInit = 10000
)

// opcodeSizes maps opcode value to total size in bytes (from QuickJS libregexp-opcode.h)
var opcodeSizes = map[OpCode]int{
	OpInvalid:                  1,
	OpChar:                     3,
	OpCharI:                    3,
	OpChar32:                   5,
	OpChar32I:                  5,
	OpDot:                      1,
	OpAny:                      1,
	OpSpace:                    1,
	OpNotSpace:                 1,
	OpLineStart:                1,
	OpLineStartM:               1,
	OpLineEnd:                  1,
	OpLineEndM:                 1,
	OpGoto:                     5,
	OpSplitGotoFirst:           5,
	OpSplitNextFirst:           5,
	OpMatch:                    1,
	OpLookaheadMatch:           1,
	OpNegativeLookaheadMatch:   1,
	OpSaveStart:                2,
	OpSaveEnd:                  2,
	OpSaveReset:                3,
	OpLoop:                     6,
	OpLoopSplitGotoFirst:       10,
	OpLoopSplitNextFirst:       10,
	OpLoopCheckAdvSplitGotoFirst: 10,
	OpLoopCheckAdvSplitNextFirst: 10,
	OpSetI32:                   6,
	OpWordBoundary:             1,
	OpWordBoundaryI:            1,
	OpNotWordBoundary:          1,
	OpNotWordBoundaryI:         1,
	OpBackReference:            2,
	OpBackReferenceI:           2,
	OpBackwardBackReference:    2,
	OpBackwardBackReferenceI:   2,
	OpRange:                    3,
	OpRangeI:                   3,
	OpRange32:                  3,
	OpRange32I:                 3,
	OpLookahead:                5,
	OpNegativeLookahead:        5,
	OpSetCharPos:               2,
	OpCheckAdvance:             2,
	OpPrev:                     1,
}// Unicode line terminators
const (
	CPLineSeparator       = 0x2028
	CPParagraphSeparator  = 0x2029
)

// Temporary buffer size
const TmpBufSize = 128

// Character class base for encoding class types
const ClassRangeBase = 0x40000000

// ============================================================================
// Opcodes (from libregexp-opcode.h)
// ============================================================================

// OpCode identifies bytecode operations
type OpCode int

const (
	OpInvalid OpCode = iota
	OpChar
	OpCharI
	OpChar32
	OpChar32I
	OpDot
	OpAny
	OpSpace
	OpNotSpace
	OpLineStart
	OpLineStartM
	OpLineEnd
	OpLineEndM
	OpGoto
	OpSplitGotoFirst
	OpSplitNextFirst
	OpMatch
	OpLookaheadMatch
	OpNegativeLookaheadMatch
	OpSaveStart
	OpSaveEnd
	OpSaveReset
	OpLoop
	OpLoopSplitGotoFirst
	OpLoopSplitNextFirst
	OpLoopCheckAdvSplitGotoFirst
	OpLoopCheckAdvSplitNextFirst
	OpSetI32
	OpWordBoundary
	OpWordBoundaryI
	OpNotWordBoundary
	OpNotWordBoundaryI
	OpBackReference
	OpBackReferenceI
	OpBackwardBackReference
	OpBackwardBackReferenceI
	OpRange
	OpRangeI
	OpRange32
	OpRange32I
	OpLookahead
	OpNegativeLookahead
	OpSetCharPos
	OpCheckAdvance
	OpPrev
	OpCount
)

// Opcode sizes
var opcodeSize = [OpCount]int{
	1,  // invalid
	3,  // char
	3,  // char_i
	5,  // char32
	5,  // char32_i
	1,  // dot
	1,  // any
	1,  // space
	1,  // not_space
	1,  // line_start
	1,  // line_start_m
	1,  // line_end
	1,  // line_end_m
	5,  // goto
	5,  // split_goto_first
	5,  // split_next_first
	1,  // match
	1,  // lookahead_match
	1,  // negative_lookahead_match
	2,  // save_start
	2,  // save_end
	3,  // save_reset
	6,  // loop
	10, // loop_split_goto_first
	10, // loop_split_next_first
	10, // loop_check_adv_split_goto_first
	10, // loop_check_adv_split_next_first
	6,  // set_i32
	1,  // word_boundary
	1,  // word_boundary_i
	1,  // not_word_boundary
	1,  // not_word_boundary_i
	2,  // back_reference
	2,  // back_reference_i
	2,  // backward_back_reference
	2,  // backward_back_reference_i
	3,  // range
	3,  // range_i
	3,  // range32
	3,  // range32_i
	5,  // lookahead
	5,  // negative_lookahead
	2,  // set_char_pos
	2,  // check_advance
	1,  // prev
}

// ============================================================================
// Bytecode Header
// ============================================================================

const (
	HeaderFlags          = 0
	HeaderCaptureCount   = 2
	HeaderRegisterCount  = 3
	HeaderBytecodeLen    = 4
	HeaderLen            = 8
)

// ============================================================================
// Character Classes
// ============================================================================

// CharRangeClass represents predefined character classes
type CharRangeClass int

const (
	CharRangeD CharRangeClass = iota
	CharRangeS
	CharRangeW
)

// Character class ranges - used for \d, \s, \w and their inverses
var charRangeD = []uint16{
	1,
	0x0030, 0x0039 + 1, // 0-9
}

var charRangeS = []uint16{
	10,
	0x0009, 0x000D + 1,  // \t-\r
	0x0020, 0x0020 + 1,  // space
	0x00A0, 0x00A0 + 1,  // non-breaking space
	0x1680, 0x1680 + 1,  // Ogham space mark
	0x2000, 0x200A + 1,  // en quad through hair space
	0x2028, 0x2029 + 1,  // line separator, paragraph separator
	0x202F, 0x202F + 1,  // narrow no-break space
	0x205F, 0x205F + 1,  // medium mathematical space
	0x3000, 0x3000 + 1,  // ideographic space
	0xFEFF, 0xFEFF + 1,  // zero width no-break space (BOM)
}

var charRangeW = []uint16{
	4,
	0x0030, 0x0039 + 1, // 0-9
	0x0041, 0x005A + 1, // A-Z
	0x005F, 0x005F + 1, // _
	0x0061, 0x007A + 1, // a-z
}

// ============================================================================
// Parse State
// ============================================================================

type parseState struct {
	byteCode          byteBuffer
	bufPtr            []byte
	bufEnd            []byte
	bufStart          []byte
	reFlags           int
	isUnicode         bool
	unicodeSets       bool
	ignoreCase        bool
	multiLine         bool
	dotAll            bool
	groupNameScope    uint8
	captureCount      int
	totalCaptureCount int // -1 = not computed yet
	hasNamedCaptures  int // -1 = don't know, 0 = no, 1 = yes
	opaque            interface{}
	groupNames        byteBuffer
	tmpBuf            [TmpBufSize]byte
	errorMsg          string
}

// ============================================================================
// Byte Buffer (DynBuf equivalent)
// ============================================================================

type byteBuffer struct {
	buf           []byte
	size          int
	allocatedSize int
	error         bool
}

func (bb *byteBuffer) init() {
	bb.buf = nil
	bb.size = 0
	bb.allocatedSize = 0
	bb.error = false
}

func (bb *byteBuffer) putC(c byte) {
	if bb.error {
		return
	}
	if len(bb.buf)-bb.size < 1 {
		if bb.claim(1) != 0 {
			return
		}
	}
	bb.buf[bb.size] = c
	bb.size++
}

func (bb *byteBuffer) putU16(val uint16) {
	if bb.error {
		return
	}
	if len(bb.buf)-bb.size < 2 {
		if bb.claim(2) != 0 {
			return
		}
	}
	cutils.PutU16(bb.buf[bb.size:], val)
	bb.size += 2
}

func (bb *byteBuffer) putU32(val uint32) {
	if bb.error {
		return
	}
	if len(bb.buf)-bb.size < 4 {
		if bb.claim(4) != 0 {
			return
		}
	}
	cutils.PutU32(bb.buf[bb.size:], val)
	bb.size += 4
}

func (bb *byteBuffer) put(data []byte) {
	if bb.error {
		return
	}
	if len(bb.buf)-bb.size < len(data) {
		if bb.claim(len(data)) != 0 {
			return
		}
	}
	copy(bb.buf[bb.size:], data)
	bb.size += len(data)
}

func (bb *byteBuffer) claim(len int) int {
	if bb.error {
		return -1
	}
	newSize := bb.size + len
	if newSize < bb.size {
		bb.error = true
		return -1
	}
	if newSize > bb.allocatedSize {
		size := bb.allocatedSize + bb.allocatedSize/2
		if size < bb.allocatedSize {
			bb.error = true
			return -1
		}
		if size < newSize {
			size = newSize
		}
		newBuf := make([]byte, size)
		if bb.buf != nil {
			copy(newBuf, bb.buf)
		}
		bb.buf = newBuf
		bb.allocatedSize = size
	}
	return 0
}

func (bb *byteBuffer) insert(pos, len int) int {
	if bb.error {
		return -1
	}
	newSize := bb.size + len
	if newSize < bb.size {
		bb.error = true
		return -1
	}
	if newSize > bb.allocatedSize {
		newAlloc := bb.allocatedSize + bb.allocatedSize/2
		if newAlloc < bb.allocatedSize {
			bb.error = true
			return -1
		}
		if newAlloc < newSize {
			newAlloc = newSize
		}
		newBuf := make([]byte, newAlloc)
		if bb.buf != nil {
			copy(newBuf, bb.buf)
		}
		bb.buf = newBuf
		bb.allocatedSize = newAlloc
	}
	// Move existing data to make room
	copy(bb.buf[pos+len:], bb.buf[pos:])
	bb.size = newSize
	return 0
}

func (bb *byteBuffer) bytes() []byte {
	return bb.buf[:bb.size]
}

func (bb *byteBuffer) len() int {
	return bb.size
}

func (bb *byteBuffer) err() bool {
	return bb.error
}

func (bb *byteBuffer) free() {
	bb.buf = nil
	bb.size = 0
	bb.allocatedSize = 0
	bb.error = false
}

// ============================================================================
// Public API
// ============================================================================

// Compile compiles a regular expression pattern.
// Returns the compiled bytecode and an error if compilation fails.
func Compile(pattern string, flags int, opaque interface{}) ([]byte, error) {
	var bc []byte
	var errMsg string

	bc, errMsg = lreCompile(pattern, flags, opaque)
	if bc == nil {
		return nil, errors.New(errMsg)
	}
	return bc, nil
}

// Match executes a compiled regex against input and returns match indices.
// capture should be a slice of size 2 * captureCount.
// Returns RetMatch (1) if matched, RetNoMatch (0) if no match, or < 0 on error.
func Match(bc []byte, input []byte, cindex int, cbufType int, opaque interface{}, capture [][]byte) int {
	if len(capture) < 2*GetCaptureCount(bc) {
		return RetMemoryError
	}
	return lreExec(capture, bc, input, cindex, len(input), cbufType, opaque)
}

// GetCaptureCount returns the number of capture groups in the compiled regex.
func GetCaptureCount(bc []byte) int {
	return int(bc[HeaderCaptureCount])
}

// GetFlags returns the flags from the compiled regex bytecode.
func GetFlags(bc []byte) int {
	return int(cutils.GetU16(bc[HeaderFlags:]))
}

// GetGroupNames returns the named group names from compiled bytecode, or nil if none.
func GetGroupNames(bc []byte) []string {
	if (GetFlags(bc) & FlagNamedGroups) == 0 {
		return nil
	}
	bcLen := cutils.GetU32(bc[HeaderBytecodeLen:])
	namesData := bc[HeaderLen+int(bcLen):]

	var names []string
	pos := 0
	for pos < len(namesData) && namesData[pos] != 0 {
		nameEnd := pos
		for nameEnd < len(namesData) && namesData[nameEnd] != 0 {
			nameEnd++
		}
		names = append(names, string(namesData[pos:nameEnd]))
		pos = nameEnd + GroupNameTrailerLen
	}
	return names
}

// GetAllocCount returns the number of capture slots needed (2 * captures + registers).
func GetAllocCount(bc []byte) int {
	return GetCaptureCount(bc)*2 + int(bc[HeaderRegisterCount])
}

// ============================================================================
// Compilation
// ============================================================================

func lreCompile(buf string, reFlags int, opaque interface{}) ([]byte, string) {
	var s parseState
	var registerCount int
	isSticky := (reFlags & FlagSticky) != 0

	// Initialize parse state
	s.bufPtr = []byte(buf)
	s.bufEnd = s.bufPtr[len(s.bufPtr):]
	s.bufStart = s.bufPtr
	s.reFlags = reFlags
	s.isUnicode = (reFlags & (FlagUnicode | FlagUnicodeSets)) != 0
	s.ignoreCase = (reFlags & FlagIgnoreCase) != 0
	s.multiLine = (reFlags & FlagMultiline) != 0
	s.dotAll = (reFlags & FlagDotAll) != 0
	s.unicodeSets = (reFlags & FlagUnicodeSets) != 0
	s.captureCount = 1
	s.totalCaptureCount = -1
	s.hasNamedCaptures = -1
	s.opaque = opaque
	s.groupNameScope = 0
	s.byteCode.init()
	s.groupNames.init()

	// Write header (will be filled in later)
	s.byteCode.putU16(uint16(reFlags))
	s.byteCode.putC(0) // capture count
	s.byteCode.putC(0) // register count
	s.byteCode.putU32(0) // bytecode length

	// If not sticky, add .* at the beginning (non-greedy)
	// Structure: split_goto_first -> any -> goto (loop back)
	// split_goto_first: execute pattern path first, push any path to stack
	// The goto jumps back to the split instruction (not save_start)
	if !isSticky {
		s.emitOpU32(OpSplitGotoFirst, 1+5)  // split to skip 'any', push 'any' path
		s.emitOp(OpAny)                       // consume one character
		// goto: jump back to the split_goto_first instruction
		// split=5 bytes, any=1 byte, goto=5 bytes
		// offset = -(5 + 1 + 5) = -11 (C QuickJS behavior)
		s.emitGotoRel(OpGoto, -int32(5+1+5)) // jump back to split_goto_first
	}
	// save_start is AFTER the non-sticky loop to capture correct position
	s.emitOpU8(OpSaveStart, 0)

	var bc []byte

	// Parse the pattern
	if s.parseDisjunction(false) != 0 {
		goto error
	}

	s.emitOpU8(OpSaveEnd, 0)
	s.emitOp(OpMatch)

	if len(s.bufPtr) != 0 {
		s.errorMsg = "extraneous characters at the end"
		goto error
	}

	if s.byteCode.err() {
		s.errorMsg = "out of memory"
		goto error
	}

	registerCount = s.computeRegisterCount()
	if registerCount < 0 {
		s.errorMsg = "too many imbricated quantifiers"
		goto error
	}

	// Fill in header
	bc = s.byteCode.bytes()
	bc[HeaderCaptureCount] = byte(s.captureCount)
	bc[HeaderRegisterCount] = byte(registerCount)
	cutils.PutU32(bc[HeaderBytecodeLen:], uint32(len(bc)-HeaderLen))

	// Add named groups if present
	if s.groupNames.len() > (s.captureCount-1)*GroupNameTrailerLen {
		s.byteCode.put(s.groupNames.bytes())
		flags := cutils.GetU16(bc[HeaderFlags:])
		cutils.PutU16(bc[HeaderFlags:], flags|FlagNamedGroups)
	}

	return bc, ""

error:
	s.byteCode.free()
	s.groupNames.free()
	return nil, s.errorMsg
}

// ============================================================================
// Parse State Methods
// ============================================================================

func (s *parseState) emitOp(op OpCode) {
	s.byteCode.putC(byte(op))
}

func (s *parseState) emitOpU8(op OpCode, val uint8) {
	s.byteCode.putC(byte(op))
	s.byteCode.putC(val)
}

func (s *parseState) emitOpU16(op OpCode, val uint16) {
	s.byteCode.putC(byte(op))
	s.byteCode.putU16(val)
}

func (s *parseState) emitOpU32(op OpCode, val uint32) int {
	s.byteCode.putC(byte(op))
	pos := s.byteCode.len()
	s.byteCode.putU32(val)
	return pos
}

// emitOpU32Forward emits an opcode with a placeholder 32-bit offset for forward references
// Returns the position where the offset is stored for later patching
func (s *parseState) emitOpU32Forward(op OpCode) int {
	s.byteCode.putC(byte(op))
	pos := s.byteCode.len()
	s.byteCode.putU32(0) // placeholder, patched later
	return pos
}

// patchU32 patches the offset at position pos to jump to target
// Formula: offset = target_position - (offset_field_position + 4)
func (s *parseState) patchU32(pos int, target int) {
	offset := int32(target - (pos + 4))
	binary.LittleEndian.PutUint32(s.byteCode.buf[pos:], uint32(offset))
}

// emitGoto emits a goto instruction with a relative offset (for backward references)
// Formula: offset = targetPos - (goto_position + 5)
// After pc += 4 in interpreter: pc = goto_position + 4
// We want: target = pc + offset = (goto_position + 4) + offset
// So: offset = target - (goto_position + 4)
// putC increments size, so after putC: s.byteCode.size = goto_position + 1
// offset = target - (s.byteCode.size - 1 + 4) = target - (s.byteCode.size + 3)
func (s *parseState) emitGoto(op OpCode, targetPos int) {
	s.byteCode.putC(byte(op))
	// After putC, s.byteCode.size = goto_position + 1
	// After pc += 4 in interpreter: pc = goto_position + 4
	// offset = target - (goto_position + 4) = target - (s.byteCode.size - 1 + 4) = target - (s.byteCode.size + 3)
	relOffset := int32(targetPos - (s.byteCode.size + 3))
	s.byteCode.putU32(uint32(relOffset))
}

// emitGotoRel emits a goto instruction with an ALREADY-COMPUTED relative offset (for OpGoto backward refs)
func (s *parseState) emitGotoRel(op OpCode, relOffset int32) {
	s.byteCode.putC(byte(op))
	s.byteCode.putU32(uint32(relOffset))
}

// emitGotoForward emits a goto instruction with placeholder offset for forward references
// Returns the position where the offset is stored for later patching
func (s *parseState) emitGotoForward(op OpCode) int {
	s.byteCode.putC(byte(op))
	pos := s.byteCode.len()
	s.byteCode.putU32(0) // placeholder
	return pos
}

func (s *parseState) parseExpect(c byte) error {
	if len(s.bufPtr) == 0 || s.bufPtr[0] != c {
		s.errorMsg = fmt.Sprintf("expecting '%c'", c)
		return errors.New(s.errorMsg)
	}
	s.bufPtr = s.bufPtr[1:]
	return nil
}

func (s *parseState) parseDigits(allowOverflow bool) (int, error) {
	v := 0
	for len(s.bufPtr) > 0 && s.bufPtr[0] >= '0' && s.bufPtr[0] <= '9' {
		c := int(s.bufPtr[0] - '0')
		v = v*10 + c
		if v >= 0x7FFFFFFF {
			if allowOverflow {
				v = 0x7FFFFFFF
			} else {
				return -1, errors.New("overflow")
			}
		}
		s.bufPtr = s.bufPtr[1:]
	}
	return v, nil
}

// ============================================================================
// Disjunction (Alternation)
// ============================================================================

func (s *parseState) parseDisjunction(isBackwardDir bool) int {
	start := s.byteCode.len()

	if s.parseAlternative(isBackwardDir) != 0 {
		return -1
	}

	for len(s.bufPtr) > 0 && s.bufPtr[0] == '|' {
		s.bufPtr = s.bufPtr[1:]

		// Insert split before first alternative
		// Use OpSplitNextFirst: try first alternative first, push second to stack
		length := s.byteCode.len() - start
		if s.byteCode.insert(start, 5) != 0 {
			s.errorMsg = "out of memory"
			return -1
		}
		bc := s.byteCode.bytes()
		bc[start] = byte(OpSplitNextFirst)
		// offset = split(5) + first_alt(length) + goto(5) - split_opcode(1) = length + 9
		// C QuickJS uses: len + 5
		splitOffset := int32(length + 5)
		binary.LittleEndian.PutUint32(bc[start+1:], uint32(splitOffset))

		// Emit forward goto to skip second alternative
		gotoPos := s.emitGotoForward(OpGoto)

		s.groupNameScope++

		if s.parseAlternative(isBackwardDir) != 0 {
			return -1
		}

		// Patch the goto
		s.patchU32(gotoPos, s.byteCode.len())
	}

	return 0
}

// ============================================================================
// Alternative (Sequence of Terms)
// ============================================================================

func (s *parseState) parseAlternative(isBackwardDir bool) int {
	for len(s.bufPtr) > 0 && s.bufPtr[0] != '|' && s.bufPtr[0] != ')' {
		if s.parseTerm(isBackwardDir) != 0 {
			return -1
		}
	}
	return 0
}

// ============================================================================
// Term (Atom with Optional Quantifier)
// ============================================================================

func (s *parseState) parseTerm(isBackwardDir bool) int {
	if len(s.bufPtr) == 0 {
		return 0
	}

	lastAtomStart := -1
	lastCaptureCount := 0
	c := int(s.bufPtr[0])

	switch c {
	case '^':
		s.bufPtr = s.bufPtr[1:]
		if s.multiLine {
			s.emitOp(OpLineStartM)
		} else {
			s.emitOp(OpLineStart)
		}

	case '$':
		s.bufPtr = s.bufPtr[1:]
		if s.multiLine {
			s.emitOp(OpLineEndM)
		} else {
			s.emitOp(OpLineEnd)
		}

	case '.':
		s.bufPtr = s.bufPtr[1:]
		lastAtomStart = s.byteCode.len()
		lastCaptureCount = s.captureCount
		if isBackwardDir {
			s.emitOp(OpPrev)
		}
		if s.dotAll {
			s.emitOp(OpAny)
		} else {
			s.emitOp(OpDot)
		}
		if isBackwardDir {
			s.emitOp(OpPrev)
		}

	case '*', '+', '?':
		s.errorMsg = "nothing to repeat"
		return -1

	case '(':
		return s.parseGroup()

	case '[':
		lastAtomStart = s.byteCode.len()
		lastCaptureCount = s.captureCount
		if isBackwardDir {
			s.emitOp(OpPrev)
		}
		if s.parseCharClass() != 0 {
			return -1
		}
		if isBackwardDir {
			s.emitOp(OpPrev)
		}

	case '\\':
		escResult := s.parseEscapeSequence()
		if escResult < 0 {
			return -1
		}
		lastAtomStart = escResult
		lastCaptureCount = s.captureCount

	default:
		// Regular character - treat as atom
		return s.parseAtom(isBackwardDir)
	}

	// Handle quantifier
	if lastAtomStart >= 0 {
		return s.parseQuantifier(lastAtomStart, lastCaptureCount)
	}

	return 0
}

// ============================================================================
// Group Parsing
// ============================================================================

func (s *parseState) parseGroup() int {
	s.bufPtr = s.bufPtr[1:] // skip '('

	if len(s.bufPtr) == 0 {
		s.errorMsg = "unexpected end"
		return -1
	}

	if s.bufPtr[0] == '?' {
		s.bufPtr = s.bufPtr[1:]
		if len(s.bufPtr) == 0 {
			s.errorMsg = "unexpected end"
			return -1
		}

		switch s.bufPtr[0] {
		case ':':
			// Non-capturing group
			s.bufPtr = s.bufPtr[1:]
			lastAtomStart := s.byteCode.len()
			lastCaptureCount := s.captureCount
			if s.parseDisjunction(false) != 0 {
				return -1
			}
			if len(s.bufPtr) == 0 || s.bufPtr[0] != ')' {
				s.errorMsg = "expecting ')'"
				return -1
			}
			s.bufPtr = s.bufPtr[1:]
			return s.parseQuantifier(lastAtomStart, lastCaptureCount)

		case '=', '!':
			// Lookahead
			isNeg := s.bufPtr[0] == '!'
			s.bufPtr = s.bufPtr[1:]

			// Save position for patching
			pos := s.byteCode.len()
			if isNeg {
				s.emitOp(OpNegativeLookahead)
			} else {
				s.emitOp(OpLookahead)
			}
			s.byteCode.putU32(0) // placeholder

			if s.parseDisjunction(false) != 0 {
				return -1
			}
			if len(s.bufPtr) == 0 || s.bufPtr[0] != ')' {
				s.errorMsg = "expecting ')'"
				return -1
			}
			s.bufPtr = s.bufPtr[1:]
			if isNeg {
				s.emitOp(OpNegativeLookaheadMatch)
			} else {
				s.emitOp(OpLookaheadMatch)
			}

			// Patch the lookahead target
			bc := s.byteCode.bytes()
			target := s.byteCode.len() - (pos + 4)
			cutils.PutU32(bc[pos:], uint32(target))

			return 0

		case '<':
			s.bufPtr = s.bufPtr[1:]
			if len(s.bufPtr) == 0 {
				s.errorMsg = "unexpected end"
				return -1
			}
			if s.bufPtr[0] == '=' || s.bufPtr[0] == '!' {
				// Lookbehind - not yet implemented
				s.errorMsg = "lookbehind not yet implemented"
				return -1
			}
			// Named capture group
			name, err := s.parseGroupName()
			if err != nil {
				s.errorMsg = "invalid group name"
				return -1
			}

			// Add group name to names list
			s.groupNames.put([]byte(name))
			s.groupNames.putC(byte(s.groupNameScope))
			s.hasNamedCaptures = 1

			// Fall through to capture parsing

		default:
			s.errorMsg = "invalid group"
			return -1
		}
	}

	// Regular capturing group
	if s.captureCount >= CaptureCountMax {
		s.errorMsg = "too many captures"
		return -1
	}

	lastAtomStart := s.byteCode.len()
	lastCaptureCount := s.captureCount
	captureIndex := s.captureCount
	s.captureCount++

	s.emitOpU8(OpSaveStart, uint8(captureIndex))

	if s.parseDisjunction(false) != 0 {
		return -1
	}

	if len(s.bufPtr) == 0 || s.bufPtr[0] != ')' {
		s.errorMsg = "expecting ')'"
		return -1
	}
	s.bufPtr = s.bufPtr[1:]

	s.emitOpU8(OpSaveEnd, uint8(captureIndex))

	return s.parseQuantifier(lastAtomStart, lastCaptureCount)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *parseState) parseGroupName() (string, error) {
	var name []byte
	for len(s.bufPtr) > 0 && s.bufPtr[0] != '>' {
		c := s.bufPtr[0]
		if c == '\\' && len(s.bufPtr) > 1 && s.bufPtr[1] == 'u' {
			// Unicode escape in group name
			s.bufPtr = s.bufPtr[2:]
			cp, err := lreParseEscape(&s.bufPtr, 2)
			if err != nil {
				return "", err
			}
			var buf [6]byte
			n := cutils.UnicodeToUTF8(buf[:], uint32(cp))
			name = append(name, buf[:n]...)
		} else {
			name = append(name, c)
			s.bufPtr = s.bufPtr[1:]
		}
	}
	if len(name) == 0 {
		return "", errors.New("empty group name")
	}
	if len(s.bufPtr) > 0 && s.bufPtr[0] == '>' {
		s.bufPtr = s.bufPtr[1:]
	}
	return string(name), nil
}

// ============================================================================
// Escape Sequences
// parseEscapeSequence parses escape sequences and returns the atom start position
// Returns: atom_start_pos on success, -1 on error
func (s *parseState) parseEscapeSequence() int {
	s.bufPtr = s.bufPtr[1:] // skip '\'
	if len(s.bufPtr) == 0 {
		s.errorMsg = "unexpected end"
		return -1
	}

	// Track atom start BEFORE emitting anything
	atomStart := s.byteCode.len()

	c := s.bufPtr[0]
	s.bufPtr = s.bufPtr[1:]

	switch c {
	case 'b':
		if s.ignoreCase && s.isUnicode {
			s.emitOp(OpNotWordBoundaryI)
		} else {
			s.emitOp(OpNotWordBoundary)
		}
	case 'B':
		if s.ignoreCase && s.isUnicode {
			s.emitOp(OpWordBoundaryI)
		} else {
			s.emitOp(OpWordBoundary)
		}
	case 'k':
		// Named back reference
		if len(s.bufPtr) < 2 || s.bufPtr[0] != '<' {
			s.errorMsg = "expecting group name"
			return -1
		}
		s.bufPtr = s.bufPtr[1:]
		_, err := s.parseGroupName()
		if err != nil {
			s.errorMsg = "invalid group name"
			return -1
		}
		// Emit placeholder back reference
		s.emitOpU8(OpBackReference, 1)
		s.byteCode.putC(1)
	case '0':
		// Null character
		s.emitChar(0)
	case '1', '2', '3', '4', '5', '6', '7', '8', '9':
		// Back reference or octal - use parseAtom to handle
		s.bufPtr = append([]byte{c}, s.bufPtr...)
		return s.parseAtom(false)
	case 'd': // \d - digit 0-9
		s.emitOp(OpRange)
		s.byteCode.putU16(1)
		s.byteCode.putU16(0x0030)
		s.byteCode.putU16(0x0039)
	case 'D': // \D - non-digit (anything except 0-9)
		s.emitOp(OpRange)
		s.byteCode.putU16(2) // 2 ranges
		s.byteCode.putU16(0x0000)
		s.byteCode.putU16(0x002F) // before '0'
		s.byteCode.putU16(0x003A)
		s.byteCode.putU16(0xFFFF) // after ':'
	case 's': // \s - whitespace
		s.emitOp(OpSpace)
	case 'S': // \S - non-whitespace (anything except whitespace)
		s.emitOp(OpNotSpace)
	case 'w': // \w - word character: 0-9, A-Z, _, a-z
		s.emitOp(OpRange)
		s.byteCode.putU16(4) // 4 ranges
		s.byteCode.putU16(0x0030)
		s.byteCode.putU16(0x0039)
		s.byteCode.putU16(0x0041)
		s.byteCode.putU16(0x005A)
		s.byteCode.putU16(0x005F)
		s.byteCode.putU16(0x005F)
		s.byteCode.putU16(0x0061)
		s.byteCode.putU16(0x007A)
	case 'W': // \W - non-word character (everything except 0-9, A-Z, _, a-z)
		s.emitOp(OpRange)
		s.byteCode.putU16(5) // 5 ranges
		s.byteCode.putU16(0x0000) // before '0'
		s.byteCode.putU16(0x002F)
		s.byteCode.putU16(0x003A) // after '9'
		s.byteCode.putU16(0x0040)
		s.byteCode.putU16(0x005B) // after '['
		s.byteCode.putU16(0x005E)
		s.byteCode.putU16(0x0060) // after '`'
		s.byteCode.putU16(0x0060)
		s.byteCode.putU16(0x007B) // after 'z'
		s.byteCode.putU16(0xFFFF)
	case 'n':
		s.emitChar('\n')
	case 'r':
		s.emitChar('\r')
	case 't':
		s.emitChar('\t')
	default:
		// Unknown escape - treat as literal character
		s.bufPtr = append([]byte{c}, s.bufPtr...)
		return s.parseAtom(false)
	}

	return atomStart
}

func (s *parseState) parseBackRefOctal(firstDigit byte) int {
	// Parse the number
	num := int(firstDigit - '0')

	// Check if it's a back reference
	if num < s.captureCount {
		// Back reference
		if s.ignoreCase {
			s.emitOpU8(OpBackReferenceI, 1)
		} else {
			s.emitOpU8(OpBackReference, 1)
		}
		s.byteCode.putC(byte(num))
		return 0
	}

	// Check for octal
	if !s.isUnicode && num <= 7 {
		// Legacy octal escape
		c := num
		if len(s.bufPtr) > 0 && s.bufPtr[0] >= '0' && s.bufPtr[0] <= '7' {
			c = c*8 + int(s.bufPtr[0]-'0')
			s.bufPtr = s.bufPtr[1:]
			if c < 32 && len(s.bufPtr) > 0 && s.bufPtr[0] >= '0' && s.bufPtr[0] <= '7' {
				c = c*8 + int(s.bufPtr[0]-'0')
				s.bufPtr = s.bufPtr[1:]
			}
		}
		s.emitChar(c)
		return 0
	}

	// Invalid back reference
	s.errorMsg = "invalid back reference"
	return -1
}

// ============================================================================
// Atom Parsing (Character or Character Class)
// ============================================================================

func (s *parseState) parseAtom(isBackwardDir bool) int {
	lastAtomStart := s.byteCode.len()
	lastCaptureCount := s.captureCount

	if isBackwardDir {
		s.emitOp(OpPrev)
	}

	// Get character
	if len(s.bufPtr) == 0 {
		s.errorMsg = "unexpected end"
		return -1
	}

	c, err := s.getClassAtom(false)
	if err != nil {
		s.errorMsg = err.Error()
		return -1
	}

	if c >= ClassRangeBase {
		// Character class - emit appropriate opcode
		class := c - ClassRangeBase
		switch class {
		case 0: // \d
			s.emitOp(OpRange)
			s.byteCode.putU16(1)
			s.byteCode.putU16(0x0030)
			s.byteCode.putU16(0x0039)
		case 1: // \D
			s.emitOp(OpNotSpace) // Simplified
		case 2: // \s
			s.emitOp(OpSpace)
		case 3: // \S
			s.emitOp(OpNotSpace)
		case 4: // \w
			s.emitOp(OpRange)
			s.byteCode.putU16(1)
			s.byteCode.putU16(0x0030)
			s.byteCode.putU16(0x003A)
			// Plus A-Z, _, a-z - simplified
		case 5: // \W
			s.emitOp(OpNotSpace) // Simplified
		}
	} else {
		// Single character
		if s.ignoreCase {
			c = int(quickunicode.LRECanonicalize(uint32(c), s.isUnicode))
		}
		s.emitChar(c)
	}

	if isBackwardDir {
		s.emitOp(OpPrev)
	}

	// Handle quantifier
	return s.parseQuantifier(lastAtomStart, lastCaptureCount)
}

func (s *parseState) getClassAtom(inclass bool) (int, error) {
	if len(s.bufPtr) == 0 {
		return -1, errors.New("unexpected end")
	}

	c := int(s.bufPtr[0])

	switch c {
	case '\\':
		s.bufPtr = s.bufPtr[1:]
		if len(s.bufPtr) == 0 {
			return '\\', nil
		}
		c = int(s.bufPtr[0])
		s.bufPtr = s.bufPtr[1:]

		switch c {
		case 'd':
			return ClassRangeBase + int(CharRangeD), nil
		case 'D':
			return ClassRangeBase + int(CharRangeD) + 1, nil
		case 's':
			return ClassRangeBase + int(CharRangeS), nil
		case 'S':
			return ClassRangeBase + int(CharRangeS) + 1, nil
		case 'w':
			return ClassRangeBase + int(CharRangeW), nil
		case 'W':
			return ClassRangeBase + int(CharRangeW) + 1, nil
		case 'c':
			if len(s.bufPtr) == 0 {
				return '\\', nil
			}
			c1 := int(s.bufPtr[0])
			if (c1 >= 'a' && c1 <= 'z') || (c1 >= 'A' && c1 <= 'Z') ||
				((c1 >= '0' && c1 <= '9' || c1 == '_') && inclass && !s.isUnicode) {
				s.bufPtr = s.bufPtr[1:]
				return c1 & 0x1F, nil
			} else if s.isUnicode {
				return -1, errors.New("invalid escape sequence")
			}
			return '\\', nil
		case '-':
			if !inclass && s.isUnicode {
				return -1, errors.New("invalid escape sequence")
			}
			return '-', nil
		case 'n':
			return '\n', nil
		case 'r':
			return '\r', nil
		case 't':
			return '\t', nil
		case '0':
			return 0, nil
		default:
			// Try to parse as escape
			s.bufPtr = append([]byte{byte(c)}, s.bufPtr...)
			ret, err := lreParseEscape(&s.bufPtr, 0)
			if err != nil || ret < 0 {
				if s.isUnicode {
					return -1, errors.New("invalid escape sequence")
				}
				return c, nil
			}
			return ret, nil
		}

		case 0:
		return -1, errors.New("unexpected end")

	default:
		s.bufPtr = s.bufPtr[1:]
		// Handle UTF-8
		if c >= 0x80 {
			r, l := cutils.UnicodeFromUTF8(s.bufPtr, cutils.UTF8CharLenMax)
			if r < 0 {
				if s.isUnicode {
					return -1, errors.New("malformed unicode char")
				}
			} else {
				s.bufPtr = s.bufPtr[l:]
				c = int(r)
				if r > 0xFFFF && !s.isUnicode {
					return -1, errors.New("malformed unicode char")
				}
			}
		}
		return c, nil
	}
}

func (s *parseState) emitChar(c int) {
	if c <= 0xFFFF {
		if s.ignoreCase {
			s.emitOpU16(OpCharI, uint16(c))
		} else {
			s.emitOpU16(OpChar, uint16(c))
		}
	} else {
		if s.ignoreCase {
			s.emitOpU32(OpChar32I, uint32(c))
		} else {
			s.emitOpU32(OpChar32, uint32(c))
		}
	}
}

// ============================================================================
// Character Class [...]
// ============================================================================

func (s *parseState) parseCharClass() int {
	s.bufPtr = s.bufPtr[1:] // skip '['

	// Check for negated
	inverted := false
	if len(s.bufPtr) > 0 && s.bufPtr[0] == '^' {
		s.bufPtr = s.bufPtr[1:]
		inverted = true
	}

	// Parse characters until ']'
	var ranges []struct{ lo, hi uint32 }
	for len(s.bufPtr) > 0 && s.bufPtr[0] != ']' {
		c1, err := s.getClassAtom(true)
		if err != nil {
			return -1
		}

		// Check for range
		if len(s.bufPtr) > 1 && s.bufPtr[0] == '-' && s.bufPtr[1] != ']' {
			s.bufPtr = s.bufPtr[1:] // skip '-'
			c2, err := s.getClassAtom(true)
			if err != nil {
				return -1
			}
			if c2 < c1 {
				s.errorMsg = "invalid class range"
				return -1
			}
			ranges = append(ranges, struct{ lo, hi uint32 }{uint32(c1), uint32(c2 + 1)})
		} else {
			ranges = append(ranges, struct{ lo, hi uint32 }{uint32(c1), uint32(c1 + 1)})
		}
	}

	if len(s.bufPtr) == 0 {
		s.errorMsg = "unterminated character class"
		return -1
	}
	s.bufPtr = s.bufPtr[1:] // skip ']'

	// Emit range opcodes
	for _, r := range ranges {
		if s.ignoreCase {
			s.emitOp(OpRangeI)
		} else {
			s.emitOp(OpRange)
		}
		s.byteCode.putU16(1) // 1 range
		if r.hi <= 0xFFFF {
			s.byteCode.putU16(uint16(r.lo))
			s.byteCode.putU16(uint16(r.hi))
		} else {
			s.byteCode.putU32(r.lo)
			s.byteCode.putU32(r.hi)
		}
	}

	if inverted {
		// Emit negated ranges - simplified, would need full range support
	}

	return 0
}

// ============================================================================
// Quantifiers
// ============================================================================

// TODO: implement quantifier bytecode emission using computed min/max
func (s *parseState) parseQuantifier(lastAtomStart, lastCaptureCount int) int {
	if len(s.bufPtr) == 0 {
		return 0
	}

	quantMin := 0
	quantMax := 1 // default for ? is {0,1}
	isGreedy := true

	switch s.bufPtr[0] {
	case '*':
		// Zero or more: {0, max_int}
		s.bufPtr = s.bufPtr[1:]
		quantMin = 0
		quantMax = 0x7FFFFFFF // INT32_MAX
	case '+':
		// One or more: {1, max_int}
		s.bufPtr = s.bufPtr[1:]
		quantMin = 1
		quantMax = 0x7FFFFFFF
	case '?':
		// Zero or one: {0, 1}
		s.bufPtr = s.bufPtr[1:]
		quantMin = 0
		quantMax = 1
	case '{':
		// Parse {n,m} quantifier
		s.bufPtr = s.bufPtr[1:]
		minVal, err := s.parseDigits(true)
		if err != nil {
			s.errorMsg = "invalid quantifier"
			return -1
		}
		quantMin = minVal
		quantMax = minVal

		if len(s.bufPtr) > 0 && s.bufPtr[0] == ',' {
			s.bufPtr = s.bufPtr[1:]
			if len(s.bufPtr) > 0 && s.bufPtr[0] >= '0' && s.bufPtr[0] <= '9' {
				maxVal, err := s.parseDigits(true)
				if err != nil {
					s.errorMsg = "invalid quantifier"
					return -1
				}
				quantMax = maxVal
			} else {
				quantMax = 0x7FFFFFFF // {n,} means unlimited
			}
		}
		if len(s.bufPtr) == 0 || s.bufPtr[0] != '}' {
			s.errorMsg = "expecting '}'"
			return -1
		}
		s.bufPtr = s.bufPtr[1:]
	default:
		return 0
	}

	// Check for non-greedy
	if len(s.bufPtr) > 0 && s.bufPtr[0] == '?' {
		s.bufPtr = s.bufPtr[1:]
		isGreedy = false
	}

	// Calculate atom size BEFORE emitting split
	atomLen := s.byteCode.len() - lastAtomStart

	if quantMax == 0 {
		// No matches allowed, remove the atom bytecode
		s.byteCode.size = lastAtomStart
		return 0
	}

	// Simple case: exactly one match (no quantifier needed)
	if quantMin == 1 && quantMax == 1 {
		return 0
	}

	// C QuickJS structure: emit atom, then split (with negative offset), then goto, then save_end
	// For greedy: OpSplitGotoFirst tries atom first (execute pc first, push splitPc)
	// For non-greedy: OpSplitNextFirst skips atom first (execute splitPc first, push pc)
	splitOp := OpSplitGotoFirst
	if !isGreedy {
		splitOp = OpSplitNextFirst
	}

	// Emit split AFTER atom (only opcode once, then offset)
	// Split offset = -(atomLen + 4) to jump back to atom start
	// After pc+=4 at split: pc = current + 4
	// We want splitPc = lastAtomStart (atom start)
	// offset = lastAtomStart - (current + 4) = -(current + 4 - lastAtomStart) = -(atomLen + 4)
	s.emitOpU32(splitOp, uint32(-(atomLen+4)))

	// Emit goto to skip past quantifier (to save_end)
	// save_end will be at current position + 2 (after goto opcode + offset)
	// After pc+=4 at goto: pc = current + 4
	// save_end will be at current + 5
	// offset = (current + 5) - (current + 4) = 1
	s.emitOpU32(OpGoto, uint32(1))

	return 0
}

// ============================================================================
// Register Count Computation
// ============================================================================

func (s *parseState) computeRegisterCount() int {
	bc := s.byteCode.bytes()
	if len(bc) < HeaderLen {
		return 0
	}

	stackSize := 0
	maxStackSize := 0
	pos := HeaderLen

	for pos < len(bc) {
		op := OpCode(bc[pos])
		if op >= OpCount {
			break
		}
		size := opcodeSize[op]
		if pos+size > len(bc) {
			break
		}

		switch op {
		case OpSetI32, OpSetCharPos:
			stackSize++
			if stackSize > maxStackSize {
				if stackSize > RegisterCountMax {
					return -1
				}
				maxStackSize = stackSize
			}
		case OpCheckAdvance, OpLoop, OpLoopSplitGotoFirst, OpLoopSplitNextFirst:
			if stackSize > 0 {
				stackSize--
			}
		case OpLoopCheckAdvSplitGotoFirst, OpLoopCheckAdvSplitNextFirst:
			if stackSize >= 2 {
				stackSize -= 2
			}
		}
		pos += size
	}

	return maxStackSize
}

// ============================================================================
// Escape Sequence Parsing
// ============================================================================

func lreParseEscape(p *[]byte, allowUTF16 int) (int, error) {
	if len(*p) == 0 {
		return -2, nil
	}

	c := int((*p)[0])
	*p = (*p)[1:]

	switch c {
	case 'b':
		return '\b', nil
	case 'f':
		return '\f', nil
	case 'n':
		return '\n', nil
	case 'r':
		return '\r', nil
	case 't':
		return '\t', nil
	case 'v':
		return '\v', nil
	case 'x':
		if len(*p) < 2 {
			return -1, errors.New("invalid hex escape")
		}
		h0 := cutils.FromHex(int((*p)[0]))
		h1 := cutils.FromHex(int((*p)[1]))
		if h0 < 0 || h1 < 0 {
			return -1, errors.New("invalid hex escape")
		}
		*p = (*p)[2:]
		return (h0 << 4) | h1, nil
	case 'u':
		if len(*p) > 0 && (*p)[0] == '{' && allowUTF16 > 0 {
			*p = (*p)[1:]
			c = 0
			for len(*p) > 0 && (*p)[0] != '}' {
				h := cutils.FromHex(int((*p)[0]))
				if h < 0 {
					return -1, errors.New("invalid unicode escape")
				}
				c = (c << 4) | h
				if c > 0x10FFFF {
					return -1, errors.New("unicode codepoint too large")
				}
				*p = (*p)[1:]
			}
			if len(*p) == 0 {
				return -1, errors.New("unterminated unicode escape")
			}
			*p = (*p)[1:] // skip '}'
		} else {
			// 4-digit unicode escape
			if len(*p) < 4 {
				return -1, errors.New("invalid unicode escape")
			}
			c = 0
			for i := 0; i < 4; i++ {
				h := cutils.FromHex(int((*p)[i]))
				if h < 0 {
					return -1, errors.New("invalid unicode escape")
				}
				c = (c << 4) | h
			}
			*p = (*p)[4:]
		}
		return c, nil
	case '0', '1', '2', '3', '4', '5', '6', '7':
		c = c - '0'
		if allowUTF16 == 2 {
			// Only accept \0 not followed by digit
			if c != 0 || len(*p) > 0 && (*p)[0] >= '0' && (*p)[0] <= '9' {
				return -1, errors.New("invalid \\0 escape")
			}
		} else {
			// Legacy octal
			if len(*p) > 0 && (*p)[0] >= '0' && (*p)[0] <= '7' {
				c = (c << 3) | int((*p)[0]-'0')
				*p = (*p)[1:]
				if c >= 32 && len(*p) > 0 && (*p)[0] >= '0' && (*p)[0] <= '7' {
					c = (c << 3) | int((*p)[0]-'0')
					*p = (*p)[1:]
				}
			}
		}
		return c, nil
	default:
		return -2, nil // Not an escape sequence
	}
}

// ============================================================================
// Execution Engine
// ============================================================================

type execContext struct {
	cbuf             []byte
	cbufEnd          []byte
	cbufStartIndex   int  // Original starting position index
	cbufType         int // 0 = 8-bit, 1 = 16-bit, 2 = 16-bit UTF-16
	captureCount     int
	isUnicode        bool
	interruptCounter int
	opaque           interface{}
	stackBuf         []stackFrame
	stackSize        int
	staticStack      [32]stackFrame
}

type stackFrame struct {
	pc    int    // offset from start of full bytecode
	cptr  []byte
	bp    int
	state int // 0 = split, 1 = lookahead, 2 = negative lookahead
}

func lreExec(capture [][]byte, bc []byte, cbuf []byte, cindex int, clen int, cbufType int, opaque interface{}) int {
	if len(bc) < HeaderLen {
		return RetMemoryError
	}

	reFlags := GetFlags(bc)
	isUnicode := (reFlags & (FlagUnicode | FlagUnicodeSets)) != 0
	captureCount := int(bc[HeaderCaptureCount])

	// Initialize capture array
	for i := range capture {
		capture[i] = nil
	}

	// Setup context
	var ctx execContext
	ctx.cbuf = cbuf
	ctx.cbufEnd = cbuf[clen:]
	if cbufType == 1 && isUnicode {
		cbufType = 2
	}
	ctx.cbufType = cbufType
	ctx.captureCount = captureCount
	ctx.isUnicode = isUnicode
	ctx.interruptCounter = InterruptCounterInit
	ctx.opaque = opaque
	ctx.cbufStartIndex = cindex // Track starting position (always 0 for initial call)

	ctx.stackBuf = ctx.staticStack[:]
	ctx.stackSize = len(ctx.staticStack)

	cptr := cbuf[cindex:]

	// Execute
	pcOffset := 0
	return lreExecBacktrack(&ctx, capture, bc, HeaderLen + pcOffset, &cptr, cbuf)
}

func lreExecBacktrack(ctx *execContext, capture [][]byte, fullBytecode []byte, startPc int, cptr *[]byte, cbuf []byte) int {
	sp := 0
	bp := 0
	pc := startPc
	bytecodeLen := len(fullBytecode)

	for {
		if sp >= len(ctx.stackBuf)-10 {
			// Grow stack
			if ctx.stackBuf == nil || &ctx.stackBuf[0] == &ctx.staticStack[0] {
				newStack := make([]stackFrame, ctx.stackSize*3/2)
				copy(newStack, ctx.stackBuf)
				ctx.stackBuf = newStack
			} else {
				newStack := make([]stackFrame, ctx.stackSize*3/2)
				copy(newStack, ctx.stackBuf)
				ctx.stackBuf = newStack
			}
		}

		if pc < 0 || pc >= bytecodeLen {
			return RetNoMatch
		}
		op := OpCode(fullBytecode[pc])
		pc += 1

		// Ensure pc has enough bytes for this opcode
		size, ok := opcodeSizes[op]
		if !ok || (pc + (size - 1)) > bytecodeLen {
			return RetNoMatch
		}

		switch op {
		case OpMatch:
			return RetMatch

		case OpChar, OpCharI, OpChar32, OpChar32I:
			var val uint32
			if op == OpChar32 || op == OpChar32I {
				val = cutils.GetU32(fullBytecode[pc:pc+4])
				pc += 4 // advance by 4 bytes (total size 5 - 1 opcode)
			} else {
				val = uint32(cutils.GetU16(fullBytecode[pc:pc+2]))
				pc += 2 // advance by 2 bytes (total size 3 -1 opcode)
			}

			if len(*cptr) == 0 {
				goto backtrack
			}
			c := getChar(cptr, ctx.cbufType)

			if op == OpCharI || op == OpChar32I {
				c = quickunicode.LRECanonicalize(c, ctx.isUnicode)
			}

			if val != c {
				goto backtrack
			}
			// NOTE: getChar already advances the input pointer, no need to advance again

		case OpDot:
			if len(*cptr) == 0 {
				goto backtrack
			}
			c := getChar(cptr, ctx.cbufType)
			if isLineTerminator(c) {
				goto backtrack
			}
			// NOTE: getChar already advances the input pointer, no need to advance again

		case OpAny:
			if len(*cptr) == 0 {
				goto backtrack
			}
			getChar(cptr, ctx.cbufType)
			// NOTE: getChar already advances the input pointer, no need to advance again

		case OpSpace, OpNotSpace:
			if len(*cptr) == 0 {
				goto backtrack
			}
			c := getChar(cptr, ctx.cbufType)
			isSpace := quickunicode.IsSpace(c)
			if (op == OpSpace && !isSpace) || (op == OpNotSpace && isSpace) {
				goto backtrack
			}

		case OpLineStart, OpLineStartM:
			// OpLineStart: match at beginning of string (cptr at start)
			// OpLineStartM: match at beginning of string, or after line terminator in multiline mode
			if op == OpLineStart {
				// Single-line mode: match only at absolute start of string
				// We check if cptr is at the original starting position (start of string)
				currentPos := len(cbuf) - len(*cptr)
				if currentPos != ctx.cbufStartIndex {
					// Not at start of original string
					goto backtrack
				}
			} else {
				// Multiline mode: match at start of string or after line terminator
				currentPos := len(cbuf) - len(*cptr)
				if currentPos == ctx.cbufStartIndex {
					// At start of string
					continue
				}
				// Check if previous char is line terminator
				prev := peekPrevChar(cptr, ctx.cbufType)
				if !isLineTerminator(prev) {
					goto backtrack
				}
			}

		case OpLineEnd, OpLineEndM:
			// OpLineEnd: match at end of string (no more characters)
			// OpLineEndM: match at end of string, or before line terminator in multiline mode
			if op == OpLineEnd {
				// Single-line mode: match only at absolute end of string
				if len(*cptr) != 0 {
					goto backtrack
				}
			} else {
				// Multiline mode: match at end of string, or before line terminator
				if len(*cptr) == 0 {
					// End of string is always a line end
					continue
				}
				// Check if current char is line terminator
				c := peekChar(cptr, ctx.cbufType)
				if !isLineTerminator(c) {
					goto backtrack
				}
			}

		case OpSplitGotoFirst, OpSplitNextFirst:
			// Split opcodes are size 5 bytes: 1 byte opcode +4 byte signed offset
			offset := int32(cutils.GetU32(fullBytecode[pc:pc+4]))
			pc += 4 // advance past operand (4 bytes, total size 5 - 1 opcode)
			splitPc := pc + int(offset)
			if splitPc < 0 || splitPc > bytecodeLen {
				goto backtrack
			}
			var pc1 int
			if op == OpSplitNextFirst {
				// OpSplitNextFirst (0x0f, 15): execute current pc first, push splitPc to stack
				pc1 = splitPc
			} else {
				// OpSplitGotoFirst (0x0e, 14): execute splitPc (pattern match) first, push current pc to stack
				pc1 = pc
				pc = splitPc
			}

			// Push state
			ctx.stackBuf[sp] = stackFrame{pc: pc1, cptr: *cptr, bp: bp, state: 0}
			sp++
			bp = sp


		case OpGoto:
			// OpGoto is size 5 bytes: 1 byte opcode + 4 byte signed offset
			offset := int32(cutils.GetU32(fullBytecode[pc:pc+4]))
			pc += 4 // advance past operand (4 bytes, total size 5 - 1 opcode)
			newPc := pc + int(offset)
			if newPc < 0 || newPc > bytecodeLen {
				return RetNoMatch
			}
			pc = newPc

		case OpSaveStart, OpSaveEnd:
			// OpSaveStart/OpSaveEnd are size 2 bytes: 1 byte opcode + 1 byte operand
			idx := int(fullBytecode[pc])
			pc += 1 // advance by 1 byte (total size 2 - 1 opcode)
			if idx >= ctx.captureCount {
				continue
			}
			capIdx := 2*idx + int(op-OpSaveStart)
			capture[capIdx] = *cptr

		case OpSaveReset:
			val1 := int(fullBytecode[pc])
			val2 := int(fullBytecode[pc+1])
			pc += 2 // advance by 2 bytes (total size 3 - 1 opcode)

			for val := val1; val <= val2; val++ {
				capIdx := 2 * val
				capture[capIdx] = nil
				capture[capIdx+1] = nil
			}

		case OpRange, OpRangeI:
			n := int(cutils.GetU16(fullBytecode[pc:pc+2]))
			pc += 2

			if len(*cptr) == 0 {
				goto backtrack
			}
			c := getChar(cptr, ctx.cbufType)

			if op == OpRangeI {
				c = quickunicode.LRECanonicalize(c, ctx.isUnicode)
			}

			// Binary search in ranges
			low := cutils.GetU16(fullBytecode[pc:pc+2])
			if uint16(c) < low {
				goto backtrack
			}

			high := cutils.GetU16(fullBytecode[pc + (n-1)*4 + 2 : pc + (n-1)*4 + 4])
			if uint16(c) > high {
				goto backtrack
			}

			// Binary search
			lo, hi := 0, n-1
			found := false
			for lo <= hi {
				mid := (lo + hi) / 2
				low = cutils.GetU16(fullBytecode[pc + mid*4 : pc + mid*4 + 2])
				high = cutils.GetU16(fullBytecode[pc + mid*4 + 2 : pc + mid*4 + 4])
				if uint16(c) < low {
					hi = mid - 1
				} else if uint16(c) > high {
					lo = mid + 1
				} else {
					found = true
					break
				}
			}

			if !found {
				goto backtrack
			}

			pc += n*4

		case OpRange32, OpRange32I:
			n := int(cutils.GetU16(fullBytecode[pc:pc+2]))
			pc += 2

			if len(*cptr) == 0 {
				goto backtrack
			}
			c := getChar(cptr, ctx.cbufType)

			if op == OpRange32I {
				c = quickunicode.LRECanonicalize(c, ctx.isUnicode)
			}

			low := cutils.GetU32(fullBytecode[pc:pc+4])
			if c < low {
				goto backtrack
			}

			high := cutils.GetU32(fullBytecode[pc + (n-1)*8 +4 : pc + (n-1)*8 +8])
			if c > high {
				goto backtrack
			}

			pc += n*8

		case OpLookahead, OpNegativeLookahead:
			// Lookahead opcodes are size 5 bytes: 1 byte opcode +4 byte signed offset
			offset := int32(cutils.GetU32(fullBytecode[pc:pc+4]))
			pc +=4 // advance past operand

			// Save state
			targetPc := pc + int(offset)
			if targetPc <0 || targetPc > bytecodeLen {
				goto backtrack
			}
			savedCptr := *cptr

			// Execute lookahead
			result := lreExecBacktrack(ctx, capture, fullBytecode, targetPc, cptr, cbuf)

			if (op == OpLookahead && result != RetMatch) ||
				(op == OpNegativeLookahead && result == RetMatch) {
				goto backtrack
			}

			*cptr = savedCptr

		case OpLookaheadMatch, OpNegativeLookaheadMatch:
			// Successfully completed lookahead
			continue

		case OpWordBoundary, OpWordBoundaryI, OpNotWordBoundary, OpNotWordBoundaryI:
			v1 := false
			v2 := false

			// Char before
			if len(*cptr) > 0 {
				prev := peekPrevChar(cptr, ctx.cbufType)
				if prev < 256 {
					v1 = quickunicode.LREIsWordByte(uint8(prev))
				} else {
					v1 = (prev == 0x017F || prev == 0x212A)
				}
			}

			// Current char
			if len(*cptr) > 0 {
				curr := peekChar(cptr, ctx.cbufType)
				if curr < 256 {
					v2 = quickunicode.LREIsWordByte(uint8(curr))
				} else {
					v2 = (curr == 0x017F || curr == 0x212A)
				}
			}

			isBoundary := (op == OpWordBoundary || op == OpWordBoundaryI)
			expected := v1 != v2 != isBoundary

			if expected != (op == OpNotWordBoundary || op == OpNotWordBoundaryI) {
				goto backtrack
			}

		case OpBackReference, OpBackReferenceI:
			n := int(fullBytecode[pc])
			pc += 1

			// Simplified backreference handling
			if n >= ctx.captureCount {
				goto backtrack
			}

			start := capture[2*n]
			end := capture[2*n+1]
			if start == nil || end == nil {
				continue // Empty capture always matches
			}

			// Compare with captured text
			for len(start) < len(end) && len(*cptr) > 0 {
				c1 := getChar(&start, ctx.cbufType)
				c2 := getChar(cptr, ctx.cbufType)

				if op == OpBackReferenceI {
					c1 = quickunicode.LRECanonicalize(c1, ctx.isUnicode)
					c2 = quickunicode.LRECanonicalize(c2, ctx.isUnicode)
				}

				if c1 != c2 {
					goto backtrack
				}
			}

		case OpPrev:
			if len(*cptr) == 0 {
				goto backtrack
			}
			prevChar(cptr, ctx.cbufType)

		default:
			goto backtrack
		}

		continue

	backtrack:
		// Pop and restore state
		if sp == 0 {
			return RetNoMatch
		}
		sp--
		frame := ctx.stackBuf[sp]
		pc = frame.pc
		*cptr = frame.cptr
		bp = frame.bp
		continue
	}
}

// ============================================================================
// Helper Functions
// ============================================================================

func getChar(cptr *[]byte, cbufType int) uint32 {
	if cbufType == 0 {
		if len(*cptr) == 0 {
			return 0
		}
		c := uint32((*cptr)[0])
		*cptr = (*cptr)[1:]
		return c
	}
	// 16-bit or UTF-16
	if len(*cptr) < 2 {
		return 0
	}
	c := uint32(cutils.GetU16(*cptr))
	*cptr = (*cptr)[2:]
	return c
}

func peekChar(cptr *[]byte, cbufType int) uint32 {
	if cbufType == 0 {
		if len(*cptr) == 0 {
			return 0
		}
		return uint32((*cptr)[0])
	}
	if len(*cptr) < 2 {
		return 0
	}
	return uint32(cutils.GetU16(*cptr))
}

func peekPrevChar(cptr *[]byte, cbufType int) uint32 {
	if cbufType == 0 {
		if len(*cptr) == 0 {
			return 0
		}
		return uint32((*cptr)[len(*cptr)-1])
	}
	if len(*cptr) < 2 {
		return 0
	}
	return uint32(cutils.GetU16((*cptr)[len(*cptr)-2:]))
}

func prevChar(cptr *[]byte, cbufType int) {
	if cbufType == 0 {
		if len(*cptr) > 0 {
			*cptr = (*cptr)[:len(*cptr)-1]
		}
	} else {
		if len(*cptr) >= 2 {
			*cptr = (*cptr)[:len(*cptr)-2]
		}
	}
}

func isLineTerminator(c uint32) bool {
	return c == '\n' || c == '\r' || c == CPLineSeparator || c == CPParagraphSeparator
}
