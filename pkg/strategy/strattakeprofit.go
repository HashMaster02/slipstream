package strategy

import (
	"time"

	"github.com/HashMaster02/slipstream/internal/data"
	"github.com/HashMaster02/slipstream/internal/events"
	"github.com/HashMaster02/slipstream/pkg/types"
)

type TakeProfit struct {
	Watchlist    []string
	Positions    map[string]int64
	PositionSize int64
	EntryPrice   map[string]types.Price
	TakeProfit   types.Price
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

	return TakeProfit{Watchlist: symbols, Positions: positions, PositionSize: positionSize, EntryPrice: entryPrices, TakeProfit: takeProfit}
}

func (strat *TakeProfit) CalculateSignals(marketData *map[string]*data.Row) ([]events.OrderEvent, bool) {
	// TODO: Orders should be submitted directly to the execution handler (once it exists) instead of being returned as a list
	var orderEvents []events.OrderEvent = make([]events.OrderEvent, len(strat.Watchlist))

	for _, symbol := range strat.Watchlist {
		bar := (*marketData)[symbol]
		// fmt.Printf("%s current bar Close: %s\n", bar.Symbol, bar.Close)
		// fmt.Printf("%s Entry price: %s\n", bar.Symbol, strat.EntryPrice[bar.Symbol].String())

		orderEvent := events.OrderEvent{
			Type:         events.OrderEventType,
			Symbol:       bar.Symbol,
			BarTimestamp: bar.Timestamp,
		}

		if strat.Positions[bar.Symbol] > 0 {
			exitPrice := strat.EntryPrice[bar.Symbol] + strat.TakeProfit
			// fmt.Printf("%s exitPrice: %s\n", bar.Symbol, exitPrice)
			if bar.Close < exitPrice {
				continue
			}

			strat.Positions[bar.Symbol] -= strat.PositionSize
			orderEvent.PositionSize = strat.PositionSize
			orderEvent.OrderDirection = events.Sell
			orderEvent.Price = exitPrice
			orderEvent.CreatedAt = time.Now()
			// fmt.Printf("%s Exit: $%s", orderEvent.Symbol, orderEvent.Price)
			orderEvents = append(orderEvents, orderEvent)
		} else {
			strat.Positions[bar.Symbol] += strat.PositionSize
			strat.EntryPrice[bar.Symbol] = bar.Close

			orderEvent.PositionSize = strat.PositionSize
			orderEvent.OrderDirection = events.Buy
			orderEvent.Price = bar.Close
			orderEvent.CreatedAt = time.Now()
			// fmt.Printf("%s Entry: $%s\n", orderEvent.Symbol, orderEvent.Price)
			orderEvents = append(orderEvents, orderEvent)
		}
	}
	return orderEvents, len(orderEvents) > 0
}
