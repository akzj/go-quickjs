// regexp_dump_c.c - Dump C QuickJS regex bytecode for comparison
// Usage: gcc -I../quickjs-master -o dump_c dump_c.c ../quickjs-master/cutils.c ../quickjs-master/libunicode.c -lm
//        ./dump_c "pattern"

#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "libregexp.h"
#include "cutils.h"
#include "libunicode.h"

int main(int argc, char **argv) {
    if (argc < 2) {
        fprintf(stderr, "Usage: %s <pattern> [flags]\n", argv[0]);
        fprintf(stderr, "  flags: 0=default, 32=sticky, 2=ignorecase, 4=multiline, 8=dotall\n");
        return 1;
    }
    
    const char *pattern = argv[1];
    int flags = 0;
    if (argc > 2) {
        flags = atoi(argv[2]);
    }
    
    int plen;
    char error_msg[256];
    uint8_t *bc = lre_compile(&plen, error_msg, sizeof(error_msg),
                               pattern, strlen(pattern), flags, NULL);
    
    if (!bc) {
        fprintf(stderr, "Compile failed: %s\n", error_msg);
        return 1;
    }
    
    printf("=== C QuickJS Bytecode ===\n");
    printf("Pattern: %s\n", pattern);
    printf("Flags: 0x%x\n", flags);
    printf("Total length: %d bytes\n", plen);
    
    // Print header
    printf("\nHeader (8 bytes):\n");
    printf("  flags: 0x%04x\n", get_u16(bc + 0));
    printf("  capture_count: %d\n", bc[2]);
    printf("  register_count: %d\n", bc[3]);
    printf("  bytecode_len: %d\n", get_u32(bc + 4));
    
    // Print hex dump of full bytecode
    printf("\nHex dump (full):\n");
    for (int i = 0; i < plen; i++) {
        printf("%02x ", bc[i]);
        if ((i + 1) % 16 == 0) printf("\n");
        else if ((i + 1) % 8 == 0) printf(" ");
    }
    printf("\n");
    
    // Disassemble
    printf("\nDisassembly:\n");
    int pos = 0;
    int bc_len = get_u32(bc + 4);
    uint8_t *bytecode = bc + 8;
    
    while (pos < bc_len) {
        int opcode = bytecode[pos];
        printf("  [%3d] ", pos);
        
        switch(opcode) {
        case 1:  printf("char '%c' (0x%04x)\n", get_u16(bytecode+pos+1), get_u16(bytecode+pos+1)); pos += 3; break;
        case 2:  printf("char_i 0x%04x\n", get_u16(bytecode+pos+1)); pos += 3; break;
        case 3:  printf("char32 0x%08x\n", get_u32(bytecode+pos+1)); pos += 5; break;
        case 4:  printf("char32_i 0x%08x\n", get_u32(bytecode+pos+1)); pos += 5; break;
        case 5:  printf("dot\n"); pos += 1; break;
        case 6:  printf("any\n"); pos += 1; break;
        case 7:  printf("space\n"); pos += 1; break;
        case 8:  printf("not_space\n"); pos += 1; break;
        case 9:  printf("line_start\n"); pos += 1; break;
        case 10: printf("line_start_m\n"); pos += 1; break;
        case 11: printf("line_end\n"); pos += 1; break;
        case 12: printf("line_end_m\n"); pos += 1; break;
        case 13: printf("goto +%d\n", (int)get_u32(bytecode+pos+1)); pos += 5; break;
        case 14: printf("split_goto_first +%d\n", (int)get_u32(bytecode+pos+1)); pos += 5; break;
        case 15: printf("split_next_first +%d\n", (int)get_u32(bytecode+pos+1)); pos += 5; break;
        case 16: printf("match\n"); pos += 1; break;
        case 17: printf("lookahead_match\n"); pos += 1; break;
        case 18: printf("negative_lookahead_match\n"); pos += 1; break;
        case 19: printf("save_start %d\n", bytecode[pos+1]); pos += 2; break;
        case 20: printf("save_end %d\n", bytecode[pos+1]); pos += 2; break;
        case 21: printf("save_reset %d %d\n", bytecode[pos+1], bytecode[pos+2]); pos += 3; break;
        case 22: printf("loop %d +%d\n", bytecode[pos+1], (int)get_u32(bytecode+pos+2)); pos += 6; break;
        case 23: printf("loop_split_goto_first %d +%d +%d\n", bytecode[pos+1], (int)get_u32(bytecode+pos+2), (int)get_u32(bytecode+pos+6)); pos += 10; break;
        case 24: printf("loop_split_next_first %d +%d +%d\n", bytecode[pos+1], (int)get_u32(bytecode+pos+2), (int)get_u32(bytecode+pos+6)); pos += 10; break;
        case 30: printf("range n=%d\n", get_u16(bytecode+pos+1)); pos += 3 + get_u16(bytecode+pos+1)*4; break;
        case 31: printf("range_i n=%d\n", get_u16(bytecode+pos+1)); pos += 3 + get_u16(bytecode+pos+1)*4; break;
        default: printf("opcode %d\n", opcode); pos += 1; break;
        }
    }
    
    free(bc);
    return 0;
}
