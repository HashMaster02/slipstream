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
	MarketEventType EventType = iota;
	SignalEventType
)


type MarketEvent struct {
	Type  EventType
	CreatedAt time.Time
}
var _ Event = MarketEvent{}

func (m MarketEvent) Kind() EventType {
	return m.Type
}

const (
	SignalLong uint8 = iota;
	SignalShort;
	SignalExit
)

type SignalEvent struct {
	Type EventType
	Symbol string
	SignalType uint8
	CreatedAt time.Time
}
var _ Event = SignalEvent{}

func (m SignalEvent) Kind() EventType {
	return m.Type
}