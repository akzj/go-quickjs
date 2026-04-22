# Regexp Bytecode Dump Tools

Tools for comparing C QuickJS and Go QuickJS regex bytecode.

## Build C Tool

```bash
cd tools/regexp_dump
gcc -I../../quickjs-master -o dump_c dump_c.c ../../quickjs-master/cutils.c ../../quickjs-master/libunicode.c -lm
```

## Run

```bash
# Dump C bytecode
./dump_c "a+b"

# Dump Go bytecode
go run dump_go.go "a+b"

# Compare multiple patterns
./dump_c "abc" > c_abc.txt
go run dump_go.go "abc" > go_abc.txt
diff c_abc.txt go_abc.txt
```

## Flags

| Value | Flag |
|-------|------|
| 0     | default |
| 32    | sticky |
| 2     | ignorecase |
| 4     | multiline |
| 8     | dotall |
