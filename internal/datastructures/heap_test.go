package datastructures_test

import (
	"container/heap"
	"sort"
	"testing"
	"time"

	"github.com/HashMaster02/slipstream/internal/data"
	"github.com/HashMaster02/slipstream/internal/datastructures"
)

// entry builds a ReaderEntry whose Row timestamp is t seconds past a fixed
// epoch. The symbol is included so ordering of equal timestamps can be checked.
func entry(symbol string, seconds int) *datastructures.ReaderEntry {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	return &datastructures.ReaderEntry{
		Row:    data.Row{Timestamp: base.Add(time.Duration(seconds) * time.Second)},
		Symbol: symbol,
	}
}

func TestReaderHeap_Len(t *testing.T) {
	tests := []struct {
		name string
		h    datastructures.ReaderHeap
		want int
	}{
		{"empty", datastructures.ReaderHeap{}, 0},
		{"nil", nil, 0},
		{"one", datastructures.ReaderHeap{entry("A", 0)}, 1},
		{"three", datastructures.ReaderHeap{entry("A", 0), entry("B", 1), entry("C", 2)}, 3},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.h.Len(); got != tc.want {
				t.Errorf("Len() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestReaderHeap_Less(t *testing.T) {
	earlier := entry("A", 0)
	later := entry("B", 1)
	sameAsEarlier := entry("C", 0)

	h := datastructures.ReaderHeap{earlier, later, sameAsEarlier}

	t.Run("earlier timestamp is less", func(t *testing.T) {
		if !h.Less(0, 1) {
			t.Error("Less(earlier, later) = false, want true")
		}
	})

	t.Run("later timestamp is not less", func(t *testing.T) {
		if h.Less(1, 0) {
			t.Error("Less(later, earlier) = true, want false")
		}
	})

	t.Run("equal timestamps are not less (strict ordering)", func(t *testing.T) {
		// Neither direction should report "less" when timestamps are equal,
		// otherwise heap ordering would be unstable/inconsistent.
		if h.Less(0, 2) {
			t.Error("Less(equal, equal) = true, want false")
		}
		if h.Less(2, 0) {
			t.Error("Less(equal, equal) = true, want false")
		}
	})
}

func TestReaderHeap_Swap(t *testing.T) {
	a := entry("A", 0)
	b := entry("B", 1)
	h := datastructures.ReaderHeap{a, b}

	h.Swap(0, 1)

	if h[0] != b || h[1] != a {
		t.Errorf("Swap did not exchange elements: got [%s, %s], want [B, A]", h[0].Symbol, h[1].Symbol)
	}

	t.Run("swap element with itself is a no-op", func(t *testing.T) {
		h := datastructures.ReaderHeap{a, b}
		h.Swap(0, 0)
		if h[0] != a || h[1] != b {
			t.Error("Swap(i, i) altered the slice")
		}
	})
}

func TestReaderHeap_Push(t *testing.T) {
	h := &datastructures.ReaderHeap{}
	e := entry("A", 5)

	h.Push(e)

	if h.Len() != 1 {
		t.Fatalf("Len() after Push = %d, want 1", h.Len())
	}
	if (*h)[0] != e {
		t.Error("Push did not append the entry")
	}

	t.Run("appends onto existing entries", func(t *testing.T) {
		second := entry("B", 6)
		h.Push(second)
		if h.Len() != 2 {
			t.Fatalf("Len() after second Push = %d, want 2", h.Len())
		}
		if (*h)[1] != second {
			t.Error("Push did not append to the end")
		}
	})
}

func TestReaderHeap_Pop(t *testing.T) {
	a := entry("A", 0)
	b := entry("B", 1)
	c := entry("C", 2)
	h := datastructures.ReaderHeap{a, b, c}

	got := h.Pop()

	if got != c {
		t.Errorf("Pop() returned the wrong entry, got %v, want C", got)
	}
	if h.Len() != 2 {
		t.Errorf("Len() after Pop = %d, want 2", h.Len())
	}
	if h[0] != a || h[1] != b {
		t.Error("Pop did not preserve the remaining entries")
	}

	t.Run("clears the popped slot to avoid memory leak", func(t *testing.T) {
		// Pop nils out the last slot before shrinking; the underlying array
		// (cap is unchanged by reslicing) should hold nil there now.
		h := datastructures.ReaderHeap{a, b}
		_ = h.Pop()
		full := h[:cap(h)]
		if full[1] != nil {
			t.Error("Pop did not nil out the vacated slot")
		}
	})

	t.Run("pop down to empty", func(t *testing.T) {
		h := datastructures.ReaderHeap{a}
		got := h.Pop()
		if got != a {
			t.Errorf("Pop() = %v, want A", got)
		}
		if h.Len() != 0 {
			t.Errorf("Len() after popping last = %d, want 0", h.Len())
		}
	})
}

// TestReaderHeap_HeapInterface exercises the type through container/heap to
// confirm the methods compose into a correct min-heap ordered by timestamp.
func TestReaderHeap_HeapInterface(t *testing.T) {
	t.Run("pops in timestamp order", func(t *testing.T) {
		// Insert deliberately out of order.
		seconds := []int{5, 1, 3, 0, 4, 2}
		h := &datastructures.ReaderHeap{}
		heap.Init(h)
		for i, s := range seconds {
			heap.Push(h, entry(string(rune('A'+i)), s))
		}

		var gotOrder []int
		for h.Len() > 0 {
			e := heap.Pop(h).(*datastructures.ReaderEntry)
			gotOrder = append(gotOrder, e.Row.Timestamp.Second())
		}

		want := append([]int(nil), seconds...)
		sort.Ints(want)
		for i := range want {
			if gotOrder[i] != want[i] {
				t.Fatalf("pop order = %v, want %v", gotOrder, want)
			}
		}
	})

	t.Run("equal timestamps all pop out", func(t *testing.T) {
		h := &datastructures.ReaderHeap{}
		heap.Init(h)
		for i := 0; i < 4; i++ {
			heap.Push(h, entry(string(rune('A'+i)), 0))
		}

		seen := map[string]bool{}
		for h.Len() > 0 {
			e := heap.Pop(h).(*datastructures.ReaderEntry)
			seen[e.Symbol] = true
		}
		for _, s := range []string{"A", "B", "C", "D"} {
			if !seen[s] {
				t.Errorf("entry %s was not popped", s)
			}
		}
	})

	t.Run("single element", func(t *testing.T) {
		h := &datastructures.ReaderHeap{}
		heap.Push(h, entry("only", 7))
		if h.Len() != 1 {
			t.Fatalf("Len() = %d, want 1", h.Len())
		}
		e := heap.Pop(h).(*datastructures.ReaderEntry)
		if e.Symbol != "only" {
			t.Errorf("popped Symbol = %q, want %q", e.Symbol, "only")
		}
		if h.Len() != 0 {
			t.Errorf("Len() after pop = %d, want 0", h.Len())
		}
	})

	t.Run("fixes ordering after a key changes", func(t *testing.T) {
		first := entry("A", 1)
		second := entry("B", 2)
		h := &datastructures.ReaderHeap{}
		heap.Push(h, first)
		heap.Push(h, second)

		// Mutate the root to a later time and re-establish heap invariant.
		first.Row.Timestamp = first.Row.Timestamp.Add(10 * time.Second)
		heap.Fix(h, 0)

		min := heap.Pop(h).(*datastructures.ReaderEntry)
		if min.Symbol != "B" {
			t.Errorf("after Fix, min Symbol = %q, want %q", min.Symbol, "B")
		}
	})
}
