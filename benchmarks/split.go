package main

import (
	"bytes"
	"fmt"
	"os"
)

func hash(x uint64) uint64 {
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return x
}

func isSpace(b byte) bool {
	return b == ' ' || (b >= 9 && b <= 13)
}

func main() {
	mode := "byte"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	const N = 500_000_000
	const NEEDLE byte = 7

	data := make([]byte, N)
	for i := 0; i < N; i++ {
		data[i] = byte(hash(uint64(i)) & 0xff)
	}

	var seps int
	switch mode {
	case "byte":
		// bytes.Count dispatches to hand-written SIMD (NEON on arm64,
		// AVX2/SSE on amd64); that's the idiomatic fast primitive.
		seps = bytes.Count(data, []byte{NEEDLE})
	case "predicate":
		// No stdlib equivalent for a custom predicate, so scalar loop.
		for _, b := range data {
			if isSpace(b) {
				seps++
			}
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown mode: %s\n", mode)
		os.Exit(1)
	}
	fmt.Println(seps + 1)
}
