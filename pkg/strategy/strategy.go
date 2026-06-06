package strategy

import (
	"github.com/HashMaster02/slipstream/internal/data"
	"github.com/HashMaster02/slipstream/internal/datastructures"
	"github.com/HashMaster02/slipstream/internal/events"
)

type Strategy interface {
	CalculateSignals(*datastructures.RowHeap, *map[string][]*data.Row) (events.OrderEvent, bool)
}
