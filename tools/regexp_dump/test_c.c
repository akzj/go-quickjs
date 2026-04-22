// test_c.c - Simple C test for libregexp with memory stubs
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "libregexp.h"
#include "cutils.h"
#include "libunicode.h"

// Memory allocation stubs required by libregexp
void *lre_realloc(void * opaque, void * ptr, size_t size) {
    return realloc(ptr, size);
}

int lre_check_stack_overflow(void * opaque, size_t alloca_size) {
    return 0;
}

int lre_check_timeout(void * opaque) {
    return 0;
}

void dump_bytecode(uint8_t *bc, int plen) {
    printf("Hex dump:\n");
    for (int i = 0; i < plen; i++) {
        printf("%02x ", bc[i]);
        if ((i + 1) % 16 == 0) printf("\n");
    }
    printf("\n\n");
    
    // Disassemble
    printf("Disassembly:\n");
    int pos = 8; // Skip header
    int bc_len = get_u32(bc + 4);
    
    while (pos - 8 < bc_len) {
        int opcode = bc[pos];
        printf("  [%3d] ", pos - 8);
        
        switch(opcode) {
        case 1:  printf("char '%c'\n", get_u16(bc+pos+1)); pos += 3; break;
        case 6:  printf("any\n"); pos += 1; break;
        case 13: printf("goto +%d\n", (int)get_u32(bc+pos+1)); pos += 5; break;
        case 14: printf("split_goto_first +%d\n", (int)get_u32(bc+pos+1)); pos += 5; break;
        case 15: printf("split_next_first +%d\n", (int)get_u32(bc+pos+1)); pos += 5; break;
        case 16: printf("match\n"); pos += 1; break;
        case 19: printf("save_start %d\n", bc[pos+1]); pos += 2; break;
        case 20: printf("save_end %d\n", bc[pos+1]); pos += 2; break;
        default: printf("opcode %d\n", opcode); pos += 1; break;
        }
    }
}

int main(int argc, char **argv) {
    int plen;
    char error_msg[256];
    
    const char *patterns[] = {"abc", "a+b", "test|best"};
    for (int i = 0; i < 3; i++) {
        uint8_t *bc = lre_compile(&plen, error_msg, sizeof(error_msg),
                                   patterns[i], strlen(patterns[i]), 0, NULL);
        
        if (!bc) {
            fprintf(stderr, "Compile '%s' failed: %s\n", patterns[i], error_msg);
            continue;
        }
        
        printf("=== C QuickJS Pattern: %s ===\n", patterns[i]);
        printf("Capture count: %d\n", bc[2]);
        dump_bytecode(bc, plen);
        free(bc);
    }
    
    return 0;
}
