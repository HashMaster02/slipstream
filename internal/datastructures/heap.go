package datastructures

import (
	"github.com/HashMaster02/slipstream/internal/data"
)

type ReaderEntry struct {
	Row data.Row
	R *data.Reader
	Symbol string
}

type ReaderHeap []*ReaderEntry

func (h ReaderHeap) Len() int {
	return len(h)
}

func (h ReaderHeap) Less(i, j int) bool {
	return h[i].Row.Timestamp.Before(h[j].Row.Timestamp)
}

func (h ReaderHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *ReaderHeap) Push(x any) {
	*h = append(*h, x.(*ReaderEntry))
}

func (h *ReaderHeap) Pop() any {
    old := *h
    n := len(old)
    x := old[n-1]
	old[n-1] = nil
    *h = old[:n-1]
    return x
}