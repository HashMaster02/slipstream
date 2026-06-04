package events

import (
	"fmt"
	"time"

	"github.com/HashMaster02/slipstream/pkg/types"
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
	OrderEventType
)


type MarketEvent struct {
	Type  EventType
	CreatedAt time.Time
}
var _ Event = MarketEvent{}

func (m MarketEvent) Kind() EventType {
	return m.Type
}

type OrderTypeType int8;
const (
	Buy OrderTypeType = iota;
	Sell
)

type OrderEvent struct {
	Type EventType
	Symbol string
	OrderType OrderTypeType
	Price types.Price
	PositionSize int64
	BarTimestamp time.Time
	CreatedAt time.Time
}
var _ Event = OrderEvent{}

func (m OrderEvent) Kind() EventType {
	return m.Type
}
func (m OrderTypeType) String() string {
	switch m {
	case Buy:
		return "Buy"
	case Sell:
		return "Sell"
	default:
		return fmt.Sprintf("OrderType(%d)", int8(m))
	}
}