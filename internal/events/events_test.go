package events_test

import (
	"testing"
	"time"

	"github.com/HashMaster02/slipstream/internal/events"
)

func TestEvent_String(t *testing.T) {
	tests := []struct {
		name string
		in   events.EventType
		want string
	}{
		{"market", events.MarketEventType, "MARKET"},
		{"unknown positive", events.EventType(7), "EventType(7)"},
		{"unknown max", events.EventType(127), "EventType(127)"},
		{"negative", events.EventType(-1), "EventType(-1)"},
		{"negative min", events.EventType(-128), "EventType(-128)"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in.String()
			if got != tc.want {
				t.Errorf("Event(%d).String() = %q, want %q", int8(tc.in), got, tc.want)
			}
		})
	}
}

func TestMarketEvent_IsZeroValue(t *testing.T) {
	if events.MarketEventType != 0 {
		t.Errorf("MarketEvent = %d, want 0 (iota base)", int8(events.MarketEventType))
	}
}

// Kind() reports the event's Type field verbatim, including the zero value and
// out-of-range values that don't correspond to a defined EventType.
func TestMarketEvent_Kind(t *testing.T) {
	tests := []struct {
		name string
		in   events.MarketEvent
		want events.EventType
	}{
		{"zero value", events.MarketEvent{}, events.MarketEventType},
		{"fully populated", events.MarketEvent{Type: events.MarketEventType, CreatedAt: time.Now()}, events.MarketEventType},
		{"undefined type field", events.MarketEvent{Type: events.EventType(7)}, events.EventType(7)},
		{"negative type field", events.MarketEvent{Type: events.EventType(-1)}, events.EventType(-1)},
		{"min type field", events.MarketEvent{Type: events.EventType(-128)}, events.EventType(-128)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.Kind(); got != tc.want {
				t.Errorf("MarketEvent.Kind() = %d, want %d", int8(got), int8(tc.want))
			}
		})
	}
}

// MarketEvent must satisfy the Event interface, and dispatching Kind() through
// the interface must yield the same result as the concrete method.
func TestMarketEvent_ImplementsEvent(t *testing.T) {
	var e events.Event = events.MarketEvent{}

	if got := e.Kind(); got != events.MarketEventType {
		t.Errorf("Event.Kind() = %d, want %d", int8(got), int8(events.MarketEventType))
	}
	if got := e.Kind().String(); got != "MARKET" {
		t.Errorf("Event.Kind().String() = %q, want %q", got, "MARKET")
	}
}

// Struct fields must round-trip the values they are assigned.
func TestMarketEvent_Fields(t *testing.T) {
	now := time.Now()
	m := events.MarketEvent{
		Type:      events.MarketEventType,
		CreatedAt: now,
	}

	if !m.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt = %v, want %v", m.CreatedAt, now)
	}
	if m.Type != events.MarketEventType {
		t.Errorf("Type = %d, want %d", int8(m.Type), int8(events.MarketEventType))
	}
}