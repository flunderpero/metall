package main

import (
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

func main() {
	mode := "grow"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	const N uint64 = 10_000_000

	switch mode {
	case "grow":
		const rounds = 100
		var acc uint64
		for r := 0; r < rounds; r++ {
			a := make([]uint64, 0, 16)
			for i := uint64(0); i < N; i++ {
				v := hash(i)
				a = append(a, v)
				acc += v
			}
		}
		fmt.Printf("grow: %d\n", acc)
	case "seq":
		const passes = 1000
		a := make([]uint64, N)
		for i := uint64(0); i < N; i++ {
			a[i] = hash(i)
		}
		var acc uint64
		for p := 0; p < passes; p++ {
			for _, x := range a {
				acc += x
			}
		}
		fmt.Printf("seq: %d\n", acc)
	case "random":
		const ops = N * 24
		a := make([]uint64, N)
		for i := uint64(0); i < N; i++ {
			a[i] = hash(i)
		}
		var acc uint64
		for i := uint64(0); i < ops; i++ {
			acc += a[hash(i)%N]
		}
		fmt.Printf("random: %d\n", acc)
	case "scatter":
		const ops = N * 23
		a := make([]uint64, N)
		for i := uint64(0); i < N; i++ {
			a[i] = hash(i)
		}
		for i := uint64(0); i < ops; i++ {
			a[hash(i)%N] += hash(i)
		}
		var acc uint64
		for _, x := range a {
			acc += x
		}
		fmt.Printf("scatter: %d\n", acc)
	case "drain":
		const rounds = 200
		a := make([]uint64, 0, N)
		var acc uint64
		for r := 0; r < rounds; r++ {
			a = a[:0]
			for i := uint64(0); i < N; i++ {
				a = append(a, hash(i))
			}
			for len(a) > 0 {
				acc += a[len(a)-1]
				a = a[:len(a)-1]
			}
		}
		fmt.Printf("drain: %d\n", acc)
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s\n", mode)
		os.Exit(1)
	}
}
