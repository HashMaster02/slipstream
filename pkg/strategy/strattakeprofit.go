package strategy

import (
	"slices"
	"time"

	"github.com/HashMaster02/slipstream/internal/data"
	"github.com/HashMaster02/slipstream/internal/datastructures"
	"github.com/HashMaster02/slipstream/internal/events"
	"github.com/HashMaster02/slipstream/pkg/types"
)

type TakeProfit struct {
	Watchlist      []string
	Positions      map[string]int64
	PositionSize   int64
	LastEntryPrice map[string]types.Price
	TakeProfit     types.Price
}

func NewTakeProfit(symbols []string, takeProfit types.Price, positionSize int64) TakeProfit {
	positions := make(map[string]int64)
	for _, symbol := range symbols {
		positions[symbol] = 0
	}

	entryPrices := make(map[string]types.Price)
	for _, symbol := range symbols {
		entryPrices[symbol] = 0
	}

	return TakeProfit{Watchlist: symbols, Positions: positions, PositionSize: positionSize, LastEntryPrice: entryPrices, TakeProfit: takeProfit}
}

func (strat *TakeProfit) CalculateSignals(marketData *datastructures.RowHeap, historicData *map[string][]*data.Row) (events.OrderEvent, bool) {
	bar := marketData.Peek()

	if !slices.Contains(strat.Watchlist, bar.Symbol) {
		return events.OrderEvent{}, false
	}

	orderEvent := events.OrderEvent{
		Type:         events.OrderEventType,
		Symbol:       bar.Symbol,
		BarTimestamp: bar.Timestamp,
	}

	if strat.Positions[bar.Symbol] > 0 {
		exitPrice := strat.LastEntryPrice[bar.Symbol] + strat.TakeProfit
		if bar.Close < exitPrice {
			return events.OrderEvent{}, false
		}

		strat.Positions[bar.Symbol] -= strat.PositionSize
		orderEvent.PositionSize = strat.PositionSize
		orderEvent.OrderDirection = events.Sell
		orderEvent.Price = exitPrice
		orderEvent.CreatedAt = time.Now()
		return orderEvent, true
	}

	strat.Positions[bar.Symbol] += strat.PositionSize
	strat.LastEntryPrice[bar.Symbol] = bar.Close

	orderEvent.PositionSize = strat.PositionSize
	orderEvent.OrderDirection = events.Buy
	orderEvent.Price = bar.Close
	orderEvent.CreatedAt = time.Now()
	return orderEvent, true

}
