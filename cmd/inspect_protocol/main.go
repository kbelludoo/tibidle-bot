package main

import (
	"bytes"
	"fmt"
	"os"
)

func main() {
	b, err := os.ReadFile("Tibidle_files/page-c0559dc61709cb03.js.download")
	if err != nil {
		fmt.Printf("err: %v\n", err)
		return
	}

	targets := []string{`case"welcome"`}
	for _, t := range targets {
		pos := 0
		for {
			idx := bytes.Index(b[pos:], []byte(t))
			if idx == -1 {
				break
			}
			realPos := pos + idx
			start := realPos - 50
			if start < 0 {
				start = 0
			}
			end := realPos + 250
			if end > len(b) {
				end = len(b)
			}
			fmt.Printf("=== %s ===\n%s\n\n", t, string(b[start:end]))
			pos = realPos + len(t)
			if pos >= len(b) {
				break
			}
		}
	}
}
