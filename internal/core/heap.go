package core

import (
	"container/heap"

	"github.com/HashMaster02/slipstream/internal/data"
)

type RowHeap []*data.Row

func (h RowHeap) Len() int {
	return len(h)
}

func (h RowHeap) Less(i, j int) bool {
	return h[i].Timestamp.Before(h[j].Timestamp)
}

func (h RowHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

// Push and Pop implement container/heap.Interface. They must keep the any
// signatures so heap.Push/heap.Pop accept *RowHeap. Prefer the typed
// PushEntry/PopEntry wrappers below for application code.
func (h *RowHeap) Push(x any) {
	*h = append(*h, x.(*data.Row))
}

func (h *RowHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return x
}

// PushEntry adds e to the heap, restoring heap order.
func (h *RowHeap) PushEntry(e *data.Row) {
	heap.Push(h, e)
}

// PopEntry removes and returns the earliest data.Row from the heap.
func (h *RowHeap) PopEntry() *data.Row {
	return heap.Pop(h).(*data.Row)
}

// Peek returns the earliest data.Row without removing it, or nil if the
// heap is empty.
func (h *RowHeap) Peek() *data.Row {
	old := *h
	if len(old) == 0 {
		return nil
	}
	return old[0]
}
