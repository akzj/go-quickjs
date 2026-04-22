package main

import (
    "fmt"
)

func main() {
    // C bytecode (without header)
    c := []byte{0x0e, 0x00, 0x00, 0x00, 0x13, 0x00, 0x12, 0x00, 0xfa, 0xff, 0xff, 0xff, 0x0e, 0x61, 0x00, 0x14, 0x00, 0x10}
    // Go bytecode pattern part (bytes 8 onwards)
    go_ := []byte{0x13, 0x00, 0x0f, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0d, 0xfd, 0xff, 0xff, 0xff, 0x14, 0x00, 0x10}
    
    fmt.Println("C      bytecode (17 bytes pattern):")
    fmt.Printf("       ")
    for i, b := range c {
        fmt.Printf("%02x ", b)
        if i == 9 { fmt.Printf("| ") }
    }
    fmt.Println()
    
    fmt.Println("Go     bytecode (18 bytes pattern):")
    fmt.Printf("       ")
    for i, b := range go_ {
        fmt.Printf("%02x ", b)
        if i == 9 { fmt.Printf("| ") }
    }
    fmt.Println()
    
    fmt.Println("OpCodes in C: 0x0e=OpChar, 0x13=OpSaveStart, 0x12=OpLoop, 0x0e=OpChar, 0x14=OpSaveEnd, 0x10=OpMatch")
    fmt.Println("OpCodes in Go: 0x13=OpSaveStart, 0x0f=OpSplitNextFirst, 0x0d=OpGoto, 0x14=OpSaveEnd, 0x10=OpMatch")
    
    fmt.Println("\nC structure: save_start | OpLoop | OpChar | save_end | match")
    fmt.Println("Go structure: save_start | OpSplitNextFirst | OpGoto | OpChar | save_end | match")
    
    fmt.Println("\nBUG FOUND: Go emits OpSplitNextFirst + OpGoto instead of OpLoop!")
}
