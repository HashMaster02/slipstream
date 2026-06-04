package strategy

import (
	"fmt"
	"slices"
	"time"

	"github.com/HashMaster02/slipstream/internal/data"
	"github.com/HashMaster02/slipstream/internal/datastructures"
	"github.com/HashMaster02/slipstream/internal/events"
	"github.com/HashMaster02/slipstream/pkg/types"
)

type Strategy interface {
	CalculateSignals(*datastructures.RowHeap, *map[string][]*data.Row) (events.OrderEvent, bool)
}

// Simple Long Strategy on Close
type BuyAndHoldOnClose struct {
	Watchlist []string
	Positions map[string]int64
	PositionSize int64
	EntryPrice types.Price
	TakeProfit types.Price
}

func NewBuyAndHoldOnClose(symbols []string, entryPrice, takeProfit types.Price, positionSize int64) BuyAndHoldOnClose {
	positions := make(map[string]int64)
	for _, symbol := range symbols {
		positions[symbol] = 0
	}

	return BuyAndHoldOnClose{Watchlist: symbols, Positions: positions, PositionSize: positionSize, EntryPrice: entryPrice, TakeProfit: takeProfit}
}

func (strat *BuyAndHoldOnClose) CalculateSignals(marketData *datastructures.RowHeap, historicData *map[string][]*data.Row) (events.OrderEvent, bool) {
    bar := marketData.Peek()

	if !slices.Contains(strat.Watchlist, bar.Symbol) {
		fmt.Print("Symbol not contained in watchlist")
		return events.OrderEvent{}, false
	}

	if (strat.Positions[bar.Symbol] > 0) {
		exitPrice := strat.EntryPrice + strat.TakeProfit
		// fmt.Println(exitPrice)

		if (bar.Close >= exitPrice ) {
			strat.Positions[bar.Symbol] -= strat.Positions[bar.Symbol]
			return events.OrderEvent{
				Type: events.OrderEventType,
				Symbol: bar.Symbol,
				PositionSize: strat.Positions[bar.Symbol],
				OrderType: events.Sell,
				Price: exitPrice,
				BarTimestamp: bar.Timestamp,
				CreatedAt: time.Now(),

			}, true
		}
		return events.OrderEvent{}, false
	}

	if (bar.Close <= strat.EntryPrice) {
		strat.Positions[bar.Symbol] += strat.PositionSize
		return events.OrderEvent{
			Type: events.OrderEventType,
			Symbol: bar.Symbol,
			PositionSize: strat.PositionSize,
			OrderType: events.Buy,
			Price: bar.Close,
			BarTimestamp: bar.Timestamp,
			CreatedAt: time.Now(),

		}, true
	}

	return events.OrderEvent{}, false

}
