package main

import (
	"fmt"
	"github.com/akzj/go-quickjs/pkg/regexp"
)

func main() {
	pattern := "abc"
	bc, err := regexp.Compile(pattern, 0, nil)
	if err != nil {
		fmt.Printf("Compile failed: %v\n", err)
		return
	}

	fmt.Printf("Pattern: %q\nBytecode len: %d bytes\nHex: %x\n", pattern, len(bc), bc)
}
