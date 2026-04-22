// test_escape.c - Test escape sequences
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "libregexp.h"
#include "cutils.h"
#include "libunicode.h"

void *lre_realloc(void *opaque, void *ptr, size_t size) { return realloc(ptr, size); }
int lre_check_stack_overflow(void *opaque, size_t alloca_size) { return 0; }
int lre_check_timeout(void *opaque) { return 0; }

void dump(const char *pattern) {
    int plen;
    char error_msg[256];
    uint8_t *bc = lre_compile(&plen, error_msg, sizeof(error_msg), pattern, strlen(pattern), 0, NULL);
    if (!bc) { printf("Error: %s\n", error_msg); return; }
    
    printf("Pattern: %s\n", pattern);
    printf("Hex: ");
    for (int i = 8; i < plen && i < 30; i++) printf("%02x ", bc[i]);
    printf("\n");
    
    int pos = 8, bc_len = get_u32(bc + 4);
    while (pos - 8 < bc_len) {
        printf("  [%2d] ", pos - 8);
        switch(bc[pos]) {
        case 1: printf("char '%c'\n", bc[pos+1]); pos += 3; break;
        case 6: printf("any\n"); pos += 1; break;
        case 7: printf("space\n"); pos += 1; break;
        case 8: printf("not_space\n"); pos += 1; break;
        case 13: printf("goto +%d\n", (int)get_u32(bc+pos+1)); pos += 5; break;
        case 14: printf("split_goto_first +%d\n", (int)get_u32(bc+pos+1)); pos += 5; break;
        case 15: printf("split_next_first +%d\n", (int)get_u32(bc+pos+1)); pos += 5; break;
        case 30: printf("range n=%d\n", get_u16(bc+pos+1)); pos += 3 + get_u16(bc+pos+1)*4; break;
        default: printf("opcode %d\n", bc[pos]); pos += 1; break;
        }
    }
    printf("\n");
    free(bc);
}

int main() {
    dump("\\d");
    dump("\\D");
    dump("\\s");
    dump("\\S");
    dump("\\w");
    dump("\\W");
    return 0;
}
