#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>

typedef struct {
    uint8_t *buf;
    size_t size;
    size_t allocated_size;
} ByteBuffer;

#define INT32_MAX 0x7FFFFFFF

static int byte_buffer_init(ByteBuffer *s) {
    s->buf = malloc(16);
    if (!s->buf) return -1;
    s->buf[0] = 0;
    s->allocated_size = 16;
    s->size = 0;
    return 0;
}

static void byte_buffer_free(ByteBuffer *s) {
    free(s->buf);
}

static int byte_buffer_putc(ByteBuffer *s, uint8_t c) {
    if (s->size >= s->allocated_size) {
        size_t new_size = s->allocated_size * 2;
        uint8_t *new_buf = realloc(s->buf, new_size);
        if (!new_buf) return -1;
        s->buf = new_buf;
        s->allocated_size = new_size;
    }
    s->buf[s->size++] = c;
    return 0;
}

static void byte_buffer_put_u32(ByteBuffer *s, uint32_t val) {
    byte_buffer_putc(s, val & 0xFF);
    byte_buffer_putc(s, (val >> 8) & 0xFF);
    byte_buffer_putc(s, (val >> 16) & 0xFF);
    byte_buffer_putc(s, (val >> 24) & 0xFF);
}

enum {
    REOP_CHAR = 14,
    REOP_SPLIT_GOTO_FIRST = 6,
    REOP_SPLIT_NEXT_FIRST = 15,
    REOP_GOTO = 13,
    REOP_SAVE_START = 19,
    REOP_SAVE_END = 20,
    REOP_MATCH = 16,
    REOP_LOOP = 18,
};

// Simplified regex compiler for "a+"
int compile_a_plus(ByteBuffer *bc) {
    byte_buffer_putc(bc, 0); // flags
    byte_buffer_putc(bc, 0);
    byte_buffer_putc(bc, 1); // captureCount = 1
    byte_buffer_putc(bc, 0); // registerCount = 0
    byte_buffer_put_u32(bc, 0); // bytecodeLen placeholder
    
    size_t code_start = bc->size;
    
    // save_start 0
    byte_buffer_putc(bc, REOP_SAVE_START);
    byte_buffer_putc(bc, 0);
    
    // Placeholder for OpLoop
    size_t loop_pos = bc->size;
    byte_buffer_putc(bc, 0);
    byte_buffer_putc(bc, 0);
    byte_buffer_put_u32(bc, 0);
    
    // char 'a'
    byte_buffer_putc(bc, REOP_CHAR);
    byte_buffer_putc(bc, 'a');
    byte_buffer_putc(bc, 0);
    
    // save_end 0
    byte_buffer_putc(bc, REOP_SAVE_END);
    byte_buffer_putc(bc, 0);
    
    // match
    byte_buffer_putc(bc, REOP_MATCH);
    
    // Calculate OpLoop offset
    size_t atom_start = code_start + 2; // after save_start
    size_t atom_len = loop_pos - atom_start; // OpLoop placeholder size
    size_t loop_end = loop_pos + 6;
    
    // Fill in OpLoop
    bc->buf[loop_pos] = REOP_LOOP;
    bc->buf[loop_pos + 1] = 0; // register
    
    // offset: from loop_end back to atom_start
    int offset = (int)(atom_start - loop_end);
    bc->buf[loop_pos + 2] = offset & 0xFF;
    bc->buf[loop_pos + 3] = (offset >> 8) & 0xFF;
    bc->buf[loop_pos + 4] = (offset >> 16) & 0xFF;
    bc->buf[loop_pos + 5] = (offset >> 24) & 0xFF;
    
    // Update bytecode length
    uint32_t bc_len = bc->size - code_start;
    bc->buf[4] = bc_len & 0xFF;
    bc->buf[5] = (bc_len >> 8) & 0xFF;
    bc->buf[6] = (bc_len >> 16) & 0xFF;
    bc->buf[7] = (bc_len >> 24) & 0xFF;
    
    return 0;
}

int main() {
    ByteBuffer bc;
    byte_buffer_init(&bc);
    
    compile_a_plus(&bc);
    
    printf("C bytecode for 'a+':\n");
    printf("Length: %zu bytes\n\n", bc.size);
    for (size_t i = 0; i < bc.size; i++) {
        printf("%02x ", bc.buf[i]);
    }
    printf("\n");
    
    byte_buffer_free(&bc);
    return 0;
}
