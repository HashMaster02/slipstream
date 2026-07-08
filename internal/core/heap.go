package core

import (
	"container/heap"

	"github.com/HashMaster02/slipstream/internal/data"
)

type QuoteHeap []*data.Quote

func (h QuoteHeap) Len() int {
	return len(h)
}

func (h QuoteHeap) Less(i, j int) bool {
	return h[i].Timestamp.Before(h[j].Timestamp)
}

func (h QuoteHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

// Push and Pop implement container/heap.Interface. They must keep the any
// signatures so heap.Push/heap.Pop accept *QuoteHeap. Prefer the typed
// PushEntry/PopEntry wrappers below for application code.
func (h *QuoteHeap) Push(x any) {
	*h = append(*h, x.(*data.Quote))
}

func (h *QuoteHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return x
}

// PushEntry adds e to the heap, restoring heap order.
func (h *QuoteHeap) PushEntry(e *data.Quote) {
	heap.Push(h, e)
}

// PopEntry removes and returns the earliest data.Quote from the heap.
func (h *QuoteHeap) PopEntry() *data.Quote {
	return heap.Pop(h).(*data.Quote)
}

// Peek returns the earliest data.Quote without removing it, or nil if the
// heap is empty.
func (h *QuoteHeap) Peek() *data.Quote {
	old := *h
	if len(old) == 0 {
		return nil
	}
	return old[0]
}
