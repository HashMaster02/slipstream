package strategy

import (
	"github.com/HashMaster02/slipstream/src/core"
	"github.com/HashMaster02/slipstream/src/data"
	"github.com/HashMaster02/slipstream/src/types"
)

// Tracks where a symbol is in the strategy's trade lifecycle
type posState int8

const (
	stateFlat     posState = iota // no position, no order in flight
	stateEntering                 // buy submitted, waiting for the fill
	stateLong                     // position open, watching for the exit
	stateExiting                  // sell submitted, waiting for the fill
)

type TakeProfit struct {
	Watchlist    []string
	PositionSize int64
	TakeProfit   types.Price
	state        map[string]posState
}

func NewTakeProfit(symbols []string, takeProfit types.Price, positionSize int64) TakeProfit {
	state := make(map[string]posState)
	for _, symbol := range symbols {
		state[symbol] = stateFlat
	}

	return TakeProfit{Watchlist: symbols, PositionSize: positionSize, TakeProfit: takeProfit, state: state}
}

func (strat *TakeProfit) CalculateSignals(marketData *map[string]*data.Quote, port *core.Portfolio) []types.Order {

	orders := make([]types.Order, 0, len(strat.Watchlist))

	for _, symbol := range strat.Watchlist {
		bar := (*marketData)[symbol]
		if bar == nil {
			continue
		}
		position := port.Positions[symbol]

		switch strat.state[symbol] {
		case stateFlat:
			order, err := types.NewOrder(symbol,
				types.Buy,
				types.Market,
				types.GTC,
				strat.PositionSize,
				0,
			)
			if err != nil {
				continue
			}
			orders = append(orders, order)
			strat.state[symbol] = stateEntering

		case stateEntering:
			if position != nil && position.Qty > 0 {
				strat.state[symbol] = stateLong
			}

		case stateLong:
			if position == nil || position.Qty == 0 {
				strat.state[symbol] = stateFlat
				continue
			}

			exitPrice := position.CostBasis + strat.TakeProfit
			if bar.Last < exitPrice {
				continue
			}

			order, err := types.NewOrder(symbol,
				types.Sell,
				types.Limit,
				types.GTC,
				position.Qty, 
				exitPrice,
			)
			if err != nil {
				continue
			}
			orders = append(orders, order)
			strat.state[symbol] = stateExiting

		case stateExiting:
			if position == nil || position.Qty == 0 {
				strat.state[symbol] = stateFlat
			}
		}
	}
	return orders
}
