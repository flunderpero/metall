package main

import (
	"fmt"
	"math"
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

func main() {
	mode := "fold"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	const N uint64 = 500_000_000

	switch mode {
	case "fold":
		var acc uint64
		for i := uint64(0); i < N; i++ {
			h := hash(i)
			if h%3 == 0 {
				continue
			}
			acc += h*17 + 42
		}
		fmt.Printf("fold: %d\n", acc)
	case "count":
		var cnt uint64
		for i := uint64(0); i < N; i++ {
			h := hash(i)
			if h%3 == 0 {
				continue
			}
			cnt++
		}
		fmt.Printf("count: %d\n", cnt)
	case "all":
		result := true
		for i := uint64(0); i < N; i++ {
			h := hash(i)
			if h%3 == 0 {
				continue
			}
			y := h*17 + 42
			if y >= math.MaxUint64 {
				result = false
				break
			}
		}
		fmt.Printf("all: %t\n", result)
	case "any":
		target := hash(499_999_990)*17 + 42
		result := false
		for i := uint64(0); i < N; i++ {
			h := hash(i)
			if h%3 == 0 {
				continue
			}
			y := h*17 + 42
			if y == target {
				result = true
				break
			}
		}
		fmt.Printf("any: %t\n", result)
	case "find":
		target := hash(499_999_990)*17 + 42
		var found uint64
		have := false
		for i := uint64(0); i < N; i++ {
			h := hash(i)
			if h%3 == 0 {
				continue
			}
			y := h*17 + 42
			if y == target {
				found = y
				have = true
				break
			}
		}
		if have {
			fmt.Printf("find: %d\n", found)
		} else {
			fmt.Println("find: None")
		}
	case "take":
		const TAKE uint64 = 400_000_000
		var acc uint64
		var produced uint64
		for i := uint64(0); i < N && produced < TAKE; i++ {
			h := hash(i)
			if h%3 == 0 {
				continue
			}
			acc += h*17 + 42
			produced++
		}
		fmt.Printf("take: %d\n", acc)
	case "take_while":
		threshold := hash(499_999_990)*17 + 42
		var acc uint64
		for i := uint64(0); i < N; i++ {
			h := hash(i)
			if h%3 == 0 {
				continue
			}
			y := h*17 + 42
			if y == threshold {
				break
			}
			acc += y
		}
		fmt.Printf("take_while: %d\n", acc)
	case "collect":
		buf := make([]uint64, 0, 1024)
		for i := uint64(0); i < N; i++ {
			h := hash(i)
			if h%3 == 0 {
				continue
			}
			y := h*17 + 42
			buf = append(buf, y)
		}
		fmt.Printf("collect: %d\n", len(buf))
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s\n", mode)
		os.Exit(1)
	}
}
