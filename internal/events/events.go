package events

import (
	"fmt"
	"time"
)

// EventType is a generic event. All events in the system will implement its interface.
type EventType int8

type Event interface {
	Kind() EventType
}

func (e EventType) String() string {
	switch e {
	case MarketEventType:
		return "MARKET"
	default:
		return fmt.Sprintf("EventType(%d)", int8(e))
	}
}

const (
	MarketEventType EventType = iota
)


type MarketEvent struct {
	Type  EventType
	Symbol string
	CreatedAt time.Time
}
var _ Event = MarketEvent{}

func (m MarketEvent) Kind() EventType {
	return m.Type
}
