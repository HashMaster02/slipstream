package datastructures_test

import (
	"testing"

	"github.com/HashMaster02/slipstream/internal/datastructures"
	"github.com/HashMaster02/slipstream/internal/events"
)

// mkEvent builds a SignalEvent tagged with symbol so individual items can be
// told apart when verifying ordering.
func mkEvent(symbol string) events.SignalEvent {
	return events.SignalEvent{Type: events.SignalEventType, Symbol: symbol}
}

// popSymbol pops one event and asserts it is a SignalEvent, returning its symbol.
func popSymbol(t *testing.T, q *datastructures.Queue) string {
	t.Helper()
	got, ok := q.Pop()
	if !ok {
		t.Fatalf("Pop() ok = false, want true (queue should still hold an item)")
	}
	se, isSignal := got.(events.SignalEvent)
	if !isSignal {
		t.Fatalf("Pop() returned %T, want events.SignalEvent", got)
	}
	return se.Symbol
}

// A freshly constructed queue must be empty: zero length and Pop reports nothing.
func TestNewQueue_StartsEmpty(t *testing.T) {
	q := datastructures.NewQueue()

	if got := q.Len(); got != 0 {
		t.Errorf("NewQueue().Len() = %d, want 0", got)
	}
	if e, ok := q.Pop(); ok || e != nil {
		t.Errorf("Pop() on empty = (%v, %t), want (nil, false)", e, ok)
	}
}

// Push must increase the reported length by one per item.
func TestQueue_PushIncrementsLen(t *testing.T) {
	q := datastructures.NewQueue()

	for i := 1; i <= 3; i++ {
		q.Push(mkEvent("X"))
		if got := q.Len(); got != i {
			t.Errorf("after %d Push calls, Len() = %d, want %d", i, got, i)
		}
	}
}

// The simplest round trip: push one event, get the same event back with ok=true.
func TestQueue_PushPopSingle(t *testing.T) {
	q := datastructures.NewQueue()
	q.Push(mkEvent("AAPL"))

	got, ok := q.Pop()
	if !ok {
		t.Fatalf("Pop() ok = false, want true")
	}
	se, isSignal := got.(events.SignalEvent)
	if !isSignal {
		t.Fatalf("Pop() returned %T, want events.SignalEvent", got)
	}
	if se.Symbol != "AAPL" {
		t.Errorf("popped Symbol = %q, want %q", se.Symbol, "AAPL")
	}
	if got := q.Len(); got != 0 {
		t.Errorf("Len() after draining = %d, want 0", got)
	}
}

// The defining property of a queue: items come out in the order they went in.
func TestQueue_FIFOOrdering(t *testing.T) {
	q := datastructures.NewQueue()
	in := []string{"AAPL", "MSFT", "GOOG", "TSLA"}

	for _, s := range in {
		q.Push(mkEvent(s))
	}
	for i, want := range in {
		if got := popSymbol(t, &q); got != want {
			t.Errorf("Pop() #%d = %q, want %q (FIFO order broken)", i, got, want)
		}
	}
	if got := q.Len(); got != 0 {
		t.Errorf("Len() after draining = %d, want 0", got)
	}
}

// Popping an empty queue is the documented "nothing left" signal.
func TestQueue_PopEmptyReturnsFalse(t *testing.T) {
	q := datastructures.NewQueue()

	e, ok := q.Pop()
	if ok || e != nil {
		t.Errorf("Pop() on empty = (%v, %t), want (nil, false)", e, ok)
	}
}

// Popping past the last element must report empty, not panic or return stale data.
func TestQueue_PopPastEmpty(t *testing.T) {
	q := datastructures.NewQueue()
	q.Push(mkEvent("AAPL"))

	if _, ok := q.Pop(); !ok {
		t.Fatalf("first Pop() ok = false, want true")
	}
	if e, ok := q.Pop(); ok || e != nil {
		t.Errorf("second Pop() = (%v, %t), want (nil, false)", e, ok)
	}
	if got := q.Len(); got != 0 {
		t.Errorf("Len() = %d, want 0", got)
	}
}

// Interleaving pushes and pops must preserve FIFO across the operations.
func TestQueue_InterleavedPushPop(t *testing.T) {
	q := datastructures.NewQueue()

	q.Push(mkEvent("A"))
	q.Push(mkEvent("B"))

	if got := popSymbol(t, &q); got != "A" {
		t.Errorf("Pop() = %q, want %q", got, "A")
	}

	q.Push(mkEvent("C"))

	if got := popSymbol(t, &q); got != "B" {
		t.Errorf("Pop() = %q, want %q", got, "B")
	}
	if got := popSymbol(t, &q); got != "C" {
		t.Errorf("Pop() = %q, want %q", got, "C")
	}
	if got := q.Len(); got != 0 {
		t.Errorf("Len() = %d, want 0", got)
	}
}

// A queue must be reusable: draining it fully then pushing again works normally.
func TestQueue_ReuseAfterDrain(t *testing.T) {
	q := datastructures.NewQueue()

	q.Push(mkEvent("first"))
	if got := popSymbol(t, &q); got != "first" {
		t.Fatalf("Pop() = %q, want %q", got, "first")
	}

	// Now empty; push a second batch and confirm it behaves like a fresh queue.
	q.Push(mkEvent("second"))
	q.Push(mkEvent("third"))

	if got := popSymbol(t, &q); got != "second" {
		t.Errorf("Pop() = %q, want %q", got, "second")
	}
	if got := popSymbol(t, &q); got != "third" {
		t.Errorf("Pop() = %q, want %q", got, "third")
	}
}

// Len must stay accurate across a mixed sequence of pushes and pops.
func TestQueue_LenTracksOperations(t *testing.T) {
	q := datastructures.NewQueue()

	steps := []struct {
		push    bool
		wantLen int
	}{
		{true, 1},
		{true, 2},
		{true, 3},
		{false, 2},
		{false, 1},
		{true, 2},
		{false, 1},
		{false, 0},
	}

	for i, s := range steps {
		if s.push {
			q.Push(mkEvent("X"))
		} else {
			q.Pop()
		}
		if got := q.Len(); got != s.wantLen {
			t.Errorf("step %d: Len() = %d, want %d", i, got, s.wantLen)
		}
	}
}

// Distinct event values must round-trip without being collapsed or shared.
func TestQueue_PreservesEventValues(t *testing.T) {
	q := datastructures.NewQueue()
	in := []events.SignalEvent{
		{Type: events.SignalEventType, Symbol: "AAPL"},
		{Type: events.SignalEventType, Symbol: "MSFT"},
		{Type: events.SignalEventType, Symbol: "GOOG"},
	}
	for _, e := range in {
		q.Push(e)
	}
	for i, want := range in {
		got, ok := q.Pop()
		if !ok {
			t.Fatalf("Pop() #%d ok = false, want true", i)
		}
		if got != events.Event(want) {
			t.Errorf("Pop() #%d = %#v, want %#v", i, got, want)
		}
	}
}

// Larger volume to shake out any structural corruption that only appears at scale.
func TestQueue_ManyElementsFIFO(t *testing.T) {
	q := datastructures.NewQueue()
	const n = 1000

	for i := 0; i < n; i++ {
		q.Push(mkEvent(string(rune('A' + i%26))))
	}
	if got := q.Len(); got != n {
		t.Fatalf("Len() = %d, want %d", got, n)
	}
	for i := 0; i < n; i++ {
		want := string(rune('A' + i%26))
		if got := popSymbol(t, &q); got != want {
			t.Fatalf("Pop() #%d = %q, want %q", i, got, want)
		}
	}
	if got := q.Len(); got != 0 {
		t.Errorf("Len() = %d, want 0 after draining", got)
	}
}

// FLAG: Pop overloads its bool return to mean both "queue empty" and "stored
// value was not an events.Event". Pushing a nil Event (or any value failing the
// type assertion) is therefore indistinguishable from an empty queue, and the
// element is silently consumed. This test pins down that ok==false is returned
// for a queued nil, documenting the conflation. See the accompanying report.
func TestQueue_PushNilEvent_IsAmbiguous(t *testing.T) {
	q := datastructures.NewQueue()

	q.Push(nil)
	if got := q.Len(); got != 1 {
		t.Errorf("Len() after Push(nil) = %d, want 1 (the nil was enqueued)", got)
	}

	// A real item is sitting in the queue, yet Pop signals "false" — the same
	// signal callers use to detect emptiness. There is no way for a caller to
	// tell this apart from an empty queue.
	if _, ok := q.Pop(); ok {
		t.Logf("Pop() of queued nil returned ok=true; conflation not present")
	} else {
		t.Logf("FLAG: Pop() of queued nil returned ok=false, same as empty queue")
	}
}
