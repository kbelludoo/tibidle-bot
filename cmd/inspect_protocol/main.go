package main

import (
	"bytes"
	"fmt"
	"os"
)

func main() {
	b, err := os.ReadFile("Tibidle_files/7-bdf2a14a2c135187.js.download")
	if err != nil {
		fmt.Printf("err: %v\n", err)
		return
	}

	targets := []string{"market_"}
	pos := 0
	for {
		idx := bytes.Index(b[pos:], []byte(targets[0]))
		if idx == -1 {
			break
		}
		realPos := pos + idx
		start := realPos - 30
		if start < 0 {
			start = 0
		}
		end := realPos + 50
		if end > len(b) {
			end = len(b)
		}
		fmt.Printf("%s\n", string(b[start:end]))
		pos = realPos + len(targets[0])
		if pos >= len(b) {
			break
		}
	}
}
