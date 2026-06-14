package core

import (
	"container/heap"

	"github.com/HashMaster02/slipstream/internal/data"
)

type CandleHeap []*data.Candle

func (h CandleHeap) Len() int {
	return len(h)
}

func (h CandleHeap) Less(i, j int) bool {
	return h[i].Timestamp.Before(h[j].Timestamp)
}

func (h CandleHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

// Push and Pop implement container/heap.Interface. They must keep the any
// signatures so heap.Push/heap.Pop accept *CandleHeap. Prefer the typed
// PushEntry/PopEntry wrappers below for application code.
func (h *CandleHeap) Push(x any) {
	*h = append(*h, x.(*data.Candle))
}

func (h *CandleHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return x
}

// PushEntry adds e to the heap, restoring heap order.
func (h *CandleHeap) PushEntry(e *data.Candle) {
	heap.Push(h, e)
}

// PopEntry removes and returns the earliest data.Candle from the heap.
func (h *CandleHeap) PopEntry() *data.Candle {
	return heap.Pop(h).(*data.Candle)
}

// Peek returns the earliest data.Candle without removing it, or nil if the
// heap is empty.
func (h *CandleHeap) Peek() *data.Candle {
	old := *h
	if len(old) == 0 {
		return nil
	}
	return old[0]
}
