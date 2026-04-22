// dump_go.go - Dump Go QuickJS regex bytecode for comparison
// Usage: go run dump_go.go [pattern...]

package main

import (
	"fmt"
	"os"

	"github.com/akzj/go-quickjs/pkg/regexp"
)

func dumpPattern(pattern string, flags int) {
	bc, err := regexp.Compile(pattern, flags, nil)
	if err != nil {
		fmt.Printf("Compile %q failed: %v\n", pattern, err)
		return
	}

	fmt.Printf("=== Go QuickJS Bytecode ===\n")
	fmt.Printf("Pattern: %s\n", pattern)
	fmt.Printf("Flags: 0x%x\n", regexp.GetFlags(bc))
	fmt.Printf("Capture count: %d\n", regexp.GetCaptureCount(bc))
	fmt.Printf("Total length: %d bytes\n", len(bc))

	// Header
	fmt.Printf("\nHeader (8 bytes):\n")
	fmt.Printf("  flags: 0x%04x\n", bc[0]|bc[1]<<8)
	fmt.Printf("  capture_count: %d\n", bc[2])
	fmt.Printf("  register_count: %d\n", bc[3])
	fmt.Printf("  bytecode_len: %d\n", int(bc[4])|int(bc[5])<<8|int(bc[6])<<16|int(bc[7])<<24)

	// Hex dump
	fmt.Printf("\nHex dump (full):\n")
	for i := 0; i < len(bc); i++ {
		fmt.Printf("%02x ", bc[i])
		if (i+1)%16 == 0 {
			fmt.Printf("\n")
		} else if (i+1)%8 == 0 {
			fmt.Printf(" ")
		}
	}
	fmt.Printf("\n\n")
}

func main() {
	patterns := []string{"abc", "a+b", "a*b", "a?b", "test|best"}
	flags := 0

	if len(os.Args) > 1 {
		patterns = os.Args[1:]
	}
	if len(os.Args) > 2 {
		fmt.Sscanf(os.Args[len(os.Args)-1], "%d", &flags)
	}

	for _, p := range patterns {
		dumpPattern(p, flags)
	}
}
