package unicode

import (
	"unicode"
)

// CONFIG_ALL_UNICODE is enabled as per original C code
const CONFIG_ALL_UNICODE = true

// LRE_CC_RES_LEN_MAX maximum length of case conversion result
const LRE_CC_RES_LEN_MAX = 3

// CharRange represents a sorted list of character intervals
type CharRange struct {
	Len         int // number of points, always even
	Size        int
	Points      []uint32 // sorted points, pairs are [start, end)
	MemOpaque   interface{}
	ReallocFunc func(opaque interface{}, ptr []uint32, size int) []uint32
}

// CharRangeOpEnum character range operation types
type CharRangeOpEnum int

const (
	CR_OP_UNION CharRangeOpEnum = iota
	CR_OP_INTER
	CR_OP_XOR
	CR_OP_SUB
)

// UnicodeNormalizationEnum normalization types
type UnicodeNormalizationEnum int

const (
	UNICODE_NFC UnicodeNormalizationEnum = iota
	UNICODE_NFD
	UNICODE_NFKC
	UNICODE_NFKD
)

// Character category bits
const (
	UNICODE_C_SPACE  = 1 << 0
	UNICODE_C_DIGIT  = 1 << 1
	UNICODE_C_UPPER  = 1 << 2
	UNICODE_C_LOWER  = 1 << 3
	UNICODE_C_UNDER  = 1 << 4
	UNICODE_C_DOLLAR = 1 << 5
	UNICODE_C_XDIGIT = 1 << 6
)

// lre_ctype_bits ASCII character type bits
var lre_ctype_bits = [256]uint8{
	// Prepopulated as per original C table
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x01, 0x01, 0x01, 0x01, 0x01, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x01, 0x00, 0x00, 0x00, 0x20, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x42, 0x42, 0x42, 0x42, 0x42, 0x42, 0x42, 0x42,
	0x42, 0x42, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x44, 0x44, 0x44, 0x44, 0x44, 0x44, 0x04,
	0x04, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04,
	0x04, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04, 0x04,
	0x04, 0x04, 0x04, 0x00, 0x00, 0x00, 0x00, 0x10,
	0x00, 0x48, 0x48, 0x48, 0x48, 0x48, 0x48, 0x08,
	0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08,
	0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08,
	0x08, 0x08, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00,
}


// Unicode lookup tables ported from libunicode-table.h
var (
	case_conv_table1 = [378]uint32{
		0x00209a30, 0x00309a00, 0x005a8173, 0x00601730,
		0x006c0730, 0x006f81b3, 0x00701700, 0x007c0700,
		0x007f8100, 0x00803040, 0x009801c3, 0x00988190,
		0x00990640, 0x009c9040, 0x00a481b4, 0x00a52e40,
		0x00bc0130, 0x00bc8640, 0x00bf8170, 0x00c00100,
		0x00c08130, 0x00c10440, 0x00c30130, 0x00c38240,
		0x00c48230, 0x00c58240, 0x00c70130, 0x00c78130,
		0x00c80130, 0x00c88240, 0x00c98130, 0x00ca0130,
		0x00ca8100, 0x00cb0130, 0x00cb8130, 0x00cc0240,
		0x00cd0100, 0x00cd8101, 0x00ce0130, 0x00ce8130,
		0x00cf0100, 0x00cf8130, 0x00d00640, 0x00d30130,
		0x00d38240, 0x00d48130, 0x00d60240, 0x00d70130,
		0x00d78240, 0x00d88230, 0x00d98440, 0x00db8130,
		0x00dc0240, 0x00de0240, 0x00df8100, 0x00e20350,
		0x00e38350, 0x00e50350, 0x00e69040, 0x00ee8100,
		0x00ef1240, 0x00f801b4, 0x00f88350, 0x00fa0240,
		0x00fb0130, 0x00fb8130, 0x00fc2840, 0x01100130,
		0x01111240, 0x011d0131, 0x011d8240, 0x011e8130,
		0x011f0131, 0x011f8201, 0x01208240, 0x01218130,
		0x01220130, 0x01228130, 0x01230a40, 0x01280101,
		0x01288101, 0x01290101, 0x01298100, 0x012a0100,
		0x012b0200, 0x012c8100, 0x012d8100, 0x012e0101,
		0x01300100, 0x01308101, 0x01318100, 0x01320101,
		0x01328101, 0x01330101, 0x01340100, 0x01348100,
		0x01350101, 0x01358101, 0x01360101, 0x01378100,
		0x01388101, 0x01390100, 0x013a8100, 0x013e8101,
		0x01400100, 0x01410101, 0x01418100, 0x01438101,
		0x01440100, 0x01448100, 0x01450200, 0x01460100,
		0x01490100, 0x014e8101, 0x014f0101, 0x01a28173,
		0x01b80440, 0x01bb0240, 0x01bd8300, 0x01bf8130,
		0x01c30130, 0x01c40330, 0x01c60130, 0x01c70230,
		0x01c801d0, 0x01c89130, 0x01d18930, 0x01d60100,
		0x01d68300, 0x01d801d3, 0x01d89100, 0x01e10173,
		0x01e18900, 0x01e60100, 0x01e68200, 0x01e78130,
		0x01e80173, 0x01e88173, 0x01ea8173, 0x01eb0173,
		0x01eb8100, 0x01ec1840, 0x01f80173, 0x01f88173,
		0x01f90100, 0x01f98100, 0x01fa01a0, 0x01fa8173,
		0x01fb8240, 0x01fc8130, 0x01fd0240, 0x01fe8330,
		0x02001030, 0x02082030, 0x02182000, 0x02281000,
		0x02302240, 0x02453640, 0x02600130, 0x02608e40,
		0x02678100, 0x02686040, 0x0298a630, 0x02b0a600,
		0x02c381b5, 0x08502631, 0x08638131, 0x08668131,
		0x08682b00, 0x087e8300, 0x09d05011, 0x09f80610,
		0x09fc0620, 0x0e400174, 0x0e408174, 0x0e410174,
		0x0e418174, 0x0e420174, 0x0e428174, 0x0e430174,
		0x0e438180, 0x0e440180, 0x0e448240, 0x0e482b30,
		0x0e5e8330, 0x0ebc8101, 0x0ebe8101, 0x0ec70101,
		0x0f007e40, 0x0f3f1840, 0x0f4b01b5, 0x0f4b81b6,
		0x0f4c01b6, 0x0f4c81b6, 0x0f4d01b7, 0x0f4d8180,
		0x0f4f0130, 0x0f506040, 0x0f800800, 0x0f840830,
		0x0f880600, 0x0f8c0630, 0x0f900800, 0x0f940830,
		0x0f980800, 0x0f9c0830, 0x0fa00600, 0x0fa40630,
		0x0fa801b0, 0x0fa88100, 0x0fa901d3, 0x0fa98100,
		0x0faa01d3, 0x0faa8100, 0x0fab01d3, 0x0fab8100,
		0x0fac8130, 0x0fad8130, 0x0fae8130, 0x0faf8130,
		0x0fb00800, 0x0fb40830, 0x0fb80200, 0x0fb90400,
		0x0fbb0201, 0x0fbc0201, 0x0fbd0201, 0x0fbe0201,
		0x0fc008b7, 0x0fc40867, 0x0fc808b8, 0x0fcc0868,
		0x0fd008b8, 0x0fd40868, 0x0fd80200, 0x0fd901b9,
		0x0fd981b1, 0x0fda01b9, 0x0fdb01b1, 0x0fdb81d7,
		0x0fdc0230, 0x0fdd0230, 0x0fde0161, 0x0fdf0173,
		0x0fe101b9, 0x0fe181b2, 0x0fe201ba, 0x0fe301b2,
		0x0fe381d8, 0x0fe40430, 0x0fe60162, 0x0fe80201,
		0x0fe901d0, 0x0fe981d0, 0x0feb01b0, 0x0feb81d0,
		0x0fec0230, 0x0fed0230, 0x0ff00201, 0x0ff101d3,
		0x0ff181d3, 0x0ff201ba, 0x0ff28101, 0x0ff301b0,
		0x0ff381d3, 0x0ff40231, 0x0ff50230, 0x0ff60131,
		0x0ff901ba, 0x0ff981b2, 0x0ffa01bb, 0x0ffb01b2,
		0x0ffb81d9, 0x0ffc0230, 0x0ffd0230, 0x0ffe0162,
		0x109301a0, 0x109501a0, 0x109581a0, 0x10990131,
		0x10a70101, 0x10b01031, 0x10b81001, 0x10c18240,
		0x125b1a31, 0x12681a01, 0x16003031, 0x16183001,
		0x16300240, 0x16310130, 0x16318130, 0x16320130,
		0x16328100, 0x16330100, 0x16338640, 0x16368130,
		0x16370130, 0x16378130, 0x16380130, 0x16390240,
		0x163a8240, 0x163f0230, 0x16406440, 0x16758440,
		0x16790240, 0x16802600, 0x16938100, 0x16968100,
		0x53202e40, 0x53401c40, 0x53910e40, 0x53993e40,
		0x53bc8440, 0x53be8130, 0x53bf0a40, 0x53c58240,
		0x53c68130, 0x53c80440, 0x53ca0101, 0x53cb1440,
		0x53d50130, 0x53d58130, 0x53d60130, 0x53d68130,
		0x53d70130, 0x53d80130, 0x53d88130, 0x53d90130,
		0x53d98131, 0x53da1040, 0x53e20131, 0x53e28130,
		0x53e30130, 0x53e38440, 0x53e58130, 0x53e61040,
		0x53ee0130, 0x53fa8240, 0x55a98101, 0x55b85020,
		0x7d8001b2, 0x7d8081b2, 0x7d8101b2, 0x7d8181da,
		0x7d8201da, 0x7d8281b3, 0x7d8301b3, 0x7d8981bb,
		0x7d8a01bb, 0x7d8a81bb, 0x7d8b01bc, 0x7d8b81bb,
		0x7f909a31, 0x7fa09a01, 0x82002831, 0x82142801,
		0x82582431, 0x826c2401, 0x82b80b31, 0x82be0f31,
		0x82c60731, 0x82ca0231, 0x82cb8b01, 0x82d18f01,
		0x82d98701, 0x82dd8201, 0x86403331, 0x86603301,
		0x86a81631, 0x86b81601, 0x8c502031, 0x8c602001,
		0xb7202031, 0xb7302001, 0xb7501931, 0xb75d9901,
		0xf4802231, 0xf4912201,
	}

	case_conv_table2 = [378]uint8{
		0x01, 0x00, 0x9c, 0x06, 0x07, 0x4d, 0x03, 0x04,
		0x10, 0x00, 0x8f, 0x0b, 0x00, 0x00, 0x11, 0x00,
		0x08, 0x00, 0x53, 0x4b, 0x52, 0x00, 0x53, 0x00,
		0x54, 0x00, 0x3b, 0x55, 0x56, 0x00, 0x58, 0x5a,
		0x40, 0x5f, 0x5e, 0x00, 0x47, 0x50, 0x63, 0x65,
		0x43, 0x66, 0x00, 0x68, 0x00, 0x6a, 0x00, 0x6c,
		0x00, 0x6e, 0x00, 0x70, 0x00, 0xdb, 0x81, 0x00,
		0x00, 0x00, 0x00, 0x1a, 0x00, 0x93, 0x00, 0x00,
		0x20, 0x36, 0x00, 0x28, 0x00, 0x24, 0x00, 0x24,
		0x25, 0x2d, 0x00, 0x13, 0x6d, 0x6f, 0x00, 0x29,
		0x27, 0x2a, 0x14, 0x16, 0x18, 0x1b, 0x1c, 0x41,
		0x1e, 0x42, 0x1f, 0x4e, 0x3c, 0x40, 0x22, 0x21,
		0x44, 0x21, 0x43, 0x26, 0x28, 0x27, 0x29, 0x23,
		0x2b, 0x4b, 0x2d, 0x46, 0x2f, 0x4c, 0x31, 0x4d,
		0x33, 0x47, 0x45, 0x99, 0x00, 0x00, 0x97, 0x91,
		0x7f, 0x80, 0x85, 0x86, 0x12, 0x82, 0x84, 0x78,
		0x79, 0x12, 0x7d, 0xa3, 0x7e, 0x7a, 0x7b, 0x8c,
		0x92, 0x98, 0xa6, 0xa0, 0x87, 0x00, 0x9a, 0xa1,
		0x95, 0x77, 0x33, 0x95, 0x00, 0x90, 0x00, 0x76,
		0x9b, 0x9a, 0x99, 0x98, 0x00, 0x00, 0xa0, 0x00,
		0x9e, 0x00, 0xa3, 0xa2, 0x15, 0x31, 0x32, 0x33,
		0xb7, 0xb8, 0x53, 0xac, 0xab, 0x12, 0x14, 0x1e,
		0x21, 0x22, 0x22, 0x2a, 0x34, 0x35, 0x00, 0xa8,
		0xa9, 0x39, 0x22, 0x4c, 0x00, 0x00, 0x97, 0x01,
		0x5a, 0xda, 0x1d, 0x36, 0x05, 0x00, 0xc7, 0xc6,
		0xc9, 0xc8, 0xcb, 0xca, 0xcd, 0xcc, 0xcf, 0xce,
		0xc4, 0xd8, 0x45, 0xd9, 0x42, 0xda, 0x46, 0xdb,
		0xd1, 0xd3, 0xd5, 0xd7, 0xdd, 0xdc, 0xf1, 0xf9,
		0x01, 0x11, 0x0a, 0x12, 0x80, 0x9f, 0x00, 0x21,
		0x80, 0xa3, 0xf0, 0x00, 0xc0, 0x40, 0xc6, 0x60,
		0xea, 0xde, 0xe6, 0x99, 0xc0, 0x00, 0x00, 0x06,
		0x60, 0xdf, 0x29, 0x00, 0x15, 0x12, 0x06, 0x16,
		0xfb, 0xe0, 0x09, 0x15, 0x12, 0x84, 0x0b, 0xc6,
		0x16, 0x02, 0xe2, 0x06, 0xc0, 0x40, 0x00, 0x46,
		0x60, 0xe1, 0xe3, 0x6d, 0x37, 0x38, 0x39, 0x18,
		0x17, 0x1a, 0x19, 0x00, 0x1d, 0x1c, 0x1f, 0x1e,
		0x00, 0x61, 0xba, 0x67, 0x45, 0x48, 0x00, 0x50,
		0x64, 0x4f, 0x51, 0x00, 0x00, 0x49, 0x00, 0x00,
		0x00, 0xa5, 0xa6, 0xa7, 0x00, 0x00, 0x00, 0x00,
		0x00, 0xb9, 0x00, 0x00, 0x5c, 0x00, 0x4a, 0x00,
		0x5d, 0x57, 0x59, 0x62, 0x60, 0x72, 0x6b, 0x71,
		0x52, 0x00, 0x3e, 0x69, 0xbb, 0x00, 0x5b, 0x00,
		0x25, 0x00, 0x48, 0xaa, 0x8a, 0x8b, 0x8c, 0xab,
		0xac, 0x58, 0x58, 0xaf, 0x94, 0xb0, 0x6f, 0xb2,
		0x61, 0x60, 0x63, 0x62, 0x65, 0x64, 0x6a, 0x6b,
		0x6c, 0x6d, 0x66, 0x67, 0x68, 0x69, 0x6f, 0x6e,
		0x71, 0x70, 0x73, 0x72, 0x75, 0x74, 0x77, 0x76,
		0x79, 0x78,
	}

	case_conv_ext = [58]uint16{
		0x0399, 0x0308, 0x0301, 0x03a5, 0x0313, 0x0300, 0x0342, 0x0391,
		0x0397, 0x03a9, 0x0046, 0x0049, 0x004c, 0x0053, 0x0069, 0x0307,
		0x02bc, 0x004e, 0x004a, 0x030c, 0x0535, 0x0552, 0x0048, 0x0331,
		0x0054, 0x0057, 0x030a, 0x0059, 0x0041, 0x02be, 0x1f08, 0x1f80,
		0x1f28, 0x1f90, 0x1f68, 0x1fa0, 0x1fba, 0x0386, 0x1fb3, 0x1fca,
		0x0389, 0x1fc3, 0x03a1, 0x1ffa, 0x038f, 0x1ff3, 0x0544, 0x0546,
		0x053b, 0x054e, 0x053d, 0x03b8, 0x0462, 0xa64a, 0x1e60, 0x03c9,
		0x006b, 0x00e5,
	}
)

// LRE case conversion types
const (
	LRE_CASE_UPPER = iota
	LRE_CASE_LOWER
	LRE_CASE_CAPITALIZE
)
// CRInit initializes a CharRange
func CRInit(cr *CharRange, memOpaque interface{}, reallocFunc func(opaque interface{}, ptr []uint32, size int) []uint32) {
	cr.Len = 0
	cr.Size = 0
	cr.Points = nil
	cr.MemOpaque = memOpaque
	cr.ReallocFunc = reallocFunc
}

// CRFree frees the CharRange memory
func CRFree(cr *CharRange) {
	if cr.ReallocFunc != nil {
		cr.Points = cr.ReallocFunc(cr.MemOpaque, cr.Points, 0)
	} else {
		cr.Points = nil
	}
	cr.Len = 0
	cr.Size = 0
}

// CRRealloc reallocates the CharRange points buffer to the given size
func CRRealloc(cr *CharRange, size int) int {
	var newPoints []uint32
	if cr.ReallocFunc != nil {
		newPoints = cr.ReallocFunc(cr.MemOpaque, cr.Points, size)
	} else {
		// Default realloc implementation if no custom function provided
		newPoints = make([]uint32, size)
		copy(newPoints, cr.Points)
	}
	if newPoints == nil && size != 0 {
		return -1
	}
	cr.Points = newPoints
	cr.Size = size
	return 0
}

// CRAddPoint adds a single point to the CharRange
func CRAddPoint(cr *CharRange, v uint32) int {
	if cr.Len >= cr.Size {
		newSize := cr.Size
		if newSize == 0 {
			newSize = 8
		} else {
			newSize *= 2
		}
		if err := CRRealloc(cr, newSize); err != 0 {
			return -1
		}
	}
	cr.Points[cr.Len] = v
	cr.Len++
	return 0
}

// CRAddInterval adds an interval [c1, c2] (inclusive) to the CharRange
func CRAddInterval(cr *CharRange, c1, c2 uint32) int {
	if (cr.Len + 2) > cr.Size {
		newSize := cr.Size
		if newSize == 0 {
			newSize = 8
		} else {
			newSize = max(newSize*2, cr.Len+2)
		}
		if err := CRRealloc(cr, newSize); err != 0 {
			return -1
		}
	}
	cr.Points[cr.Len] = c1
	cr.Points[cr.Len+1] = c2 + 1 // convert to [start, end) as per original
	cr.Len += 2
	return 0
}

// IsSpaceByte checks if an ASCII byte is a space character
func IsSpaceByte(c uint8) bool {
	return (lre_ctype_bits[c] & UNICODE_C_SPACE) != 0
}

// IsIDStartByte checks if an ASCII byte is a valid identifier start character
func IsIDStartByte(c uint8) bool {
	return (lre_ctype_bits[c] & (UNICODE_C_UPPER | UNICODE_C_LOWER | UNICODE_C_UNDER | UNICODE_C_DOLLAR)) != 0
}

// IsIDContinueByte checks if an ASCII byte is a valid identifier continue character
func IsIDContinueByte(c uint8) bool {
	return (lre_ctype_bits[c] & (UNICODE_C_UPPER | UNICODE_C_LOWER | UNICODE_C_UNDER | UNICODE_C_DOLLAR | UNICODE_C_DIGIT)) != 0
}

// IsWordByte checks if an ASCII byte is a word character
func IsWordByte(c uint8) bool {
	return (lre_ctype_bits[c] & (UNICODE_C_UPPER | UNICODE_C_LOWER | UNICODE_C_UNDER | UNICODE_C_DIGIT)) != 0
}

// LREIsWordByte checks if a code point is a word character (for regexp compatibility)
func LREIsWordByte(c uint8) bool {
	return IsWordByte(c)
}

// IsSpace checks if a rune is a space character
func IsSpace(c uint32) bool {
	if c == 0x00A0 {
		return true
	}
	if c < 256 {
		return IsSpaceByte(uint8(c))
	}
	return IsSpaceNonASCII(c)
}

// IsSpaceNonASCII checks if a non-ASCII rune is a space character
func IsSpaceNonASCII(c uint32) bool {
	// Match original C implementation including all JS valid whitespace
	if c == 0x00A0 || c == 0x1680 || c == 0x202F || c == 0x205F || c == 0x3000 {
		return true
	}
	if c >= 0x2000 && c <= 0x200A {
		return true
	}
	return unicode.IsSpace(rune(c))
}

// IsCased checks if a rune is cased (upper, lower, title)
func IsCased(c uint32) bool {
	r := rune(c)
	return unicode.IsUpper(r) || unicode.IsLower(r) || unicode.IsTitle(r)
}

// IsCaseIgnorable checks if a rune is case ignorable
func IsCaseIgnorable(c uint32) bool {
	r := rune(c)
	return unicode.IsMark(r) || unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
}

// IsIDStart checks if a rune is a valid identifier start character
func IsIDStart(c uint32) bool {
	if !CONFIG_ALL_UNICODE {
		return !IsSpaceNonASCII(c)
	}
	r := rune(c)
	return unicode.IsLetter(r) || r == '_' || r == '$'
}

// IsIDContinue checks if a rune is a valid identifier continue character
func IsIDContinue(c uint32) bool {
	if c >= 0x200C && c <= 0x200D {
		return true // ZWNJ and ZWJ are allowed
	}
	if !CONFIG_ALL_UNICODE {
		return !IsSpaceNonASCII(c)
	}
	r := rune(c)
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '$' || unicode.IsMark(r) || unicode.IsNumber(r)
}

// JSIsIdentFirst checks if a rune is a valid JavaScript identifier first character
func JSIsIdentFirst(c uint32) bool {
	if c < 128 {
		return IsIDStartByte(uint8(c))
	}
	return IsIDStart(c)
}

// JSIsIdentNext checks if a rune is a valid JavaScript identifier next character
func JSIsIdentNext(c uint32) bool {
	if c < 128 {
		return IsIDContinueByte(uint8(c))
	}
	if c >= 0x200C && c <= 0x200D {
		return true
	}
	return IsIDContinue(c)
}

// CRCopy copies cr1 to cr
func CRCopy(cr *CharRange, cr1 *CharRange) int {
	if CRRealloc(cr, cr1.Len) != 0 {
		return -1
	}
	copy(cr.Points, cr1.Points)
	cr.Len = cr1.Len
	return 0
}

// maxInt returns the maximum of two integers
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// CROp performs set operations on character ranges
// a_pt/a_len and b_pt/b_len are input ranges, cr is output
// Operations: UNION, INTER, XOR, SUB
func CROp(cr *CharRange, aPt []uint32, aLen int, bPt []uint32, bLen int, op CharRangeOpEnum) int {
	aIdx := 0
	bIdx := 0

	for {
		var v uint32
		var aIn, bIn bool

		// Get one more point from a or b in increasing order
		if aIdx < aLen && bIdx < bLen {
			if aPt[aIdx] < bPt[bIdx] {
				v = aPt[aIdx]
				aIdx++
				aIn = (aIdx & 1) == 1
				bIn = (bIdx & 1) == 1
			} else if aPt[aIdx] == bPt[bIdx] {
				v = aPt[aIdx]
				aIdx++
				bIdx++
				aIn = (aIdx & 1) == 1
				bIn = (bIdx & 1) == 1
			} else {
				v = bPt[bIdx]
				bIdx++
				aIn = (aIdx & 1) == 1
				bIn = (bIdx & 1) == 1
			}
		} else if aIdx < aLen {
			v = aPt[aIdx]
			aIdx++
			aIn = (aIdx & 1) == 1
			bIn = (bIdx & 1) == 1
		} else if bIdx < bLen {
			v = bPt[bIdx]
			bIdx++
			aIn = (aIdx & 1) == 1
			bIn = (bIdx & 1) == 1
		} else {
			break
		}

		// Determine if point should be added based on operation
		var isIn bool
		switch op {
		case CR_OP_UNION:
			isIn = aIn || bIn
		case CR_OP_INTER:
			isIn = aIn && bIn
		case CR_OP_XOR:
			isIn = aIn != bIn
		case CR_OP_SUB:
			isIn = aIn && !bIn
		}

		// Add point if in/out status changes
		if isIn != ((cr.Len & 1) == 1) {
			if CRAddPoint(cr, v) != 0 {
				return -1
			}
		}
	}

	// Merge consecutive intervals
	crCompress(cr)
	return 0
}

// LRECaseConv converts a rune to the specified case, returns the number of runes written to res (max 3)
func LRECaseConv(res []uint32, c uint32, convType int) int {
	// Handle ASCII case conversion first
	if c < 128 {
		switch convType {
		case LRE_CASE_UPPER:
			if c >= 'a' && c <= 'z' {
				res[0] = c - 32
				return 1
			}
		case LRE_CASE_LOWER:
			if c >= 'A' && c <= 'Z' {
				res[0] = c + 32
				return 1
			}
		case LRE_CASE_CAPITALIZE:
			if c >= 'a' && c <= 'z' {
				res[0] = c - 32
				return 1
			}
		}
		res[0] = c
		return 1
	}

	// Binary search in case_conv_table1
	low := 0
	high := len(case_conv_table1) - 1
	for low <= high {
		mid := (low + high) / 2
		entry := case_conv_table1[mid]
		code := entry >> 10
		mask := entry & 0x3ff
		if code > c {
			high = mid - 1
		} else if code+mask < c {
			low = mid + 1
		} else {
			// Found matching entry
			t2 := case_conv_table2[mid]
			offset := int(c - code)
			switch convType {
			case LRE_CASE_UPPER:
				delta := int(t2 & 0x1f)
				if (t2 >> 6) & 1 != 0 {
					// Upper case is base + offset + delta
					res[0] = c + uint32(delta)
				} else {
					res[0] = c - uint32(delta)
				}
				return 1
			case LRE_CASE_LOWER:
				delta := int((t2 >> 2) & 0x1f)
				if (t2 >> 7) & 1 != 0 {
					res[0] = c + uint32(delta)
				} else {
					res[0] = c - uint32(delta)
				}
				return 1
			case LRE_CASE_CAPITALIZE:
				// Special case for extended entries
				if (t2 & 0x80) != 0 && offset < len(case_conv_ext) {
					res[0] = uint32(case_conv_ext[offset])
					return 1
				}
				// Default to upper case for capitalize
				delta := int(t2 & 0x1f)
				if (t2 >> 6) & 1 != 0 {
					res[0] = c + uint32(delta)
				} else {
					res[0] = c - uint32(delta)
				}
				return 1
			}
		}
	}

	// No conversion found, return original
	res[0] = c
	return 1
}

// LRECanonicalize canonicalizes a rune for regex matching
func LRECanonicalize(c uint32, isUnicode bool) uint32 {
	if !isUnicode {
		// For non-unicode mode, convert ASCII to uppercase
		if c >= 'a' && c <= 'z' {
			return c - 32
		}
		return c
	}
	var res [3]uint32
	n := LRECaseConv(res[:], c, LRE_CASE_LOWER)
	if n > 0 {
		return res[0]
	}
	return c
}

// LREIsCased checks if a rune is cased (upper, lower, or title)
func LREIsCased(c uint32) bool {
	r := rune(c)
	return unicode.IsUpper(r) || unicode.IsLower(r) || unicode.IsTitle(r)
}

// LREIsCaseIgnorable checks if a rune is case ignorable
func LREIsCaseIgnorable(c uint32) bool {
	r := rune(c)
	return unicode.IsMark(r) || unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
}


// crCompress merges consecutive intervals and removes empty intervals
func crCompress(cr *CharRange) {
	pt := cr.Points
	len := cr.Len
	i := 0
	k := 0

	for (i + 1) < len {
		if pt[i] == pt[i+1] {
			// Empty interval
			i += 2
		} else {
			j := i
			for (j + 3) < len && pt[j+1] == pt[j+2] {
				j += 2
			}
			// Copy interval
			pt[k] = pt[i]
			pt[k+1] = pt[j+1]
			k += 2
			i = j + 2
		}
	}
	cr.Len = k
}

// CROp1 performs set operation using cr's existing points
func CROp1(cr *CharRange, bPt []uint32, bLen int, op CharRangeOpEnum) int {
	a := *cr
	cr.Len = 0
	cr.Size = 0
	cr.Points = nil
	ret := CROp(cr, a.Points, a.Len, bPt, bLen, op)
	CRFree(&a)
	return ret
}

// CRInvert inverts the character range
func CRInvert(cr *CharRange) int {
	length := cr.Len
	if CRRealloc(cr, length+2) != 0 {
		return -1
	}
	// Move points to make room for 0 at start
	copy(cr.Points[1:], cr.Points[:length])
	cr.Points[0] = 0
	cr.Points[length+1] = 0xFFFFFFFF
	cr.Len = length + 2
	crCompress(cr)
	return 0
}

// Case conversion run types
const (
	runTypeU int = iota
	runTypeL
	runTypeUF
	runTypeLF
	runTypeUL
	runTypeLSU
	runTypeU2L399Ext2
	runTypeUFD20
	runTypeUFD1Ext
	runTypeUExt
	runTypeLFExt
	runTypeUFExt2
	runTypeLFExt2
	runTypeUFExt3
)

// LRECaseConv converts a character according to conv_type
// conv_type: 0 = to upper, 1 = to lower, 2 = case folding
