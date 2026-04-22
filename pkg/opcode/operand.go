package opcode

// ReadI32 reads a 32-bit signed integer from bytecode
func ReadI32(code []byte, pc *int) int32 {
	v := int32(code[*pc]) |
		int32(code[*pc+1])<<8 |
		int32(code[*pc+2])<<16 |
		int32(code[*pc+3])<<24
	*pc += 4
	return v
}

// ReadU32 reads a 32-bit unsigned integer from bytecode
func ReadU32(code []byte, pc *int) uint32 {
	return uint32(ReadI32(code, pc))
}

// ReadI8 reads an 8-bit signed integer from bytecode
func ReadI8(code []byte, pc *int) int8 {
	v := int8(code[*pc])
	*pc++
	return v
}

// ReadU8 reads an 8-bit unsigned integer from bytecode
func ReadU8(code []byte, pc *int) uint8 {
	v := uint8(code[*pc])
	*pc++
	return v
}

// ReadI16 reads a 16-bit signed integer from bytecode
func ReadI16(code []byte, pc *int) int16 {
	v := int16(code[*pc]) | int16(code[*pc+1])<<8
	*pc += 2
	return v
}

// ReadU16 reads a 16-bit unsigned integer from bytecode
func ReadU16(code []byte, pc *int) uint16 {
	v := uint16(code[*pc]) | uint16(code[*pc+1])<<8
	*pc += 2
	return v
}

// WriteI32 writes a 32-bit signed integer to bytecode
func WriteI32(code *[]byte, v int32) {
	*code = append(*code,
		byte(v),
		byte(v>>8),
		byte(v>>16),
		byte(v>>24),
	)
}

// WriteU32 writes a 32-bit unsigned integer to bytecode
func WriteU32(code *[]byte, v uint32) {
	WriteI32(code, int32(v))
}

// WriteI8 writes an 8-bit signed integer to bytecode
func WriteI8(code *[]byte, v int8) {
	*code = append(*code, byte(v))
}

// WriteU8 writes an 8-bit unsigned integer to bytecode
func WriteU8(code *[]byte, v uint8) {
	*code = append(*code, v)
}
