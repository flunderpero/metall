package main

import (
	"container/heap"
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

// MaxHeap is the default container/heap usage: a slice with the five-method interface.
// Less reports h[i] > h[j] so Pop yields the largest element.
type MaxHeap []uint64

func (h MaxHeap) Len() int           { return len(h) }
func (h MaxHeap) Less(i, j int) bool { return h[i] > h[j] }
func (h MaxHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *MaxHeap) Push(x any) { *h = append(*h, x.(uint64)) }

func (h *MaxHeap) Pop() any {
	old := *h
	n := len(old)
	v := old[n-1]
	*h = old[:n-1]
	return v
}

func main() {
	mode := "sort"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	const N uint64 = 10_000_000
	const K = 1024

	switch mode {
	case "sort":
		h := make(MaxHeap, 0, N)
		for i := uint64(0); i < N; i++ {
			heap.Push(&h, hash(i))
		}
		var acc uint64
		for h.Len() > 0 {
			acc = acc*1000003 + heap.Pop(&h).(uint64)
		}
		fmt.Printf("sort: %d\n", acc)
	case "churn":
		const M = N * 3
		h := make(MaxHeap, 0, K+1)
		var acc uint64
		for i := uint64(0); i < M; i++ {
			heap.Push(&h, hash(i))
			if h.Len() > K {
				acc = acc*1000003 + heap.Pop(&h).(uint64)
			}
		}
		for h.Len() > 0 {
			acc = acc*1000003 + heap.Pop(&h).(uint64)
		}
		fmt.Printf("churn: %d\n", acc)
	case "pushpop":
		const M = N * 3
		h := make(MaxHeap, 0, K)
		for i := uint64(0); i < K; i++ {
			heap.Push(&h, hash(i))
		}
		var acc uint64
		for i := uint64(0); i < M; i++ {
			top := heap.Pop(&h).(uint64)
			acc = acc*1000003 + top
			heap.Push(&h, hash(K+i))
		}
		for h.Len() > 0 {
			acc = acc*1000003 + heap.Pop(&h).(uint64)
		}
		fmt.Printf("pushpop: %d\n", acc)
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s\n", mode)
		os.Exit(1)
	}
}
