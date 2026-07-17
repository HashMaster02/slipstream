package strategy

import (
	"github.com/HashMaster02/slipstream/src/core"
	"github.com/HashMaster02/slipstream/src/data"
	"github.com/HashMaster02/slipstream/src/types"
)

type TakeProfit struct {
	Watchlist    []string
	PositionSize int64
	EntryPrice   map[string]types.Price
	TakeProfit   types.Price
}

func NewTakeProfit(symbols []string, takeProfit types.Price, positionSize int64) TakeProfit {
	entryPrices := make(map[string]types.Price)
	for _, symbol := range symbols {
		entryPrices[symbol] = 0
	}

	return TakeProfit{Watchlist: symbols, PositionSize: positionSize, EntryPrice: entryPrices, TakeProfit: takeProfit}
}

func (strat *TakeProfit) CalculateSignals(marketData *map[string]*data.Quote, port *core.Portfolio) []types.Order {

	orders := make([]types.Order, 0, len(strat.Watchlist))

	for _, symbol := range strat.Watchlist {
		bar := (*marketData)[symbol]
		position, succ := port.Positions[bar.Symbol] // TODO: Fix a race condition that occurs here occasionally

		if !succ {
			// Init a new position within the portfolio
			position = &core.Position{
				Symbol:            symbol,
				Qty:               0,
				CurrentSharePrice: types.PriceFromFloat(0),
				CostBasis:         types.PriceFromFloat(0),
			}
			port.Positions[symbol] = position
		}

		if position.Qty == 0 {
			// Buy signal
			position.CurrentSharePrice = bar.Last

			order, err := types.NewOrder(bar.Symbol,
							types.Buy,
							types.Market,
							types.GTC,
							strat.PositionSize,
							bar.Last,
						)
			if err != nil {
				return orders
			}
			orders = append(orders, order)

		} else {
			// Sell signal
			exitPrice := position.CostBasis + strat.TakeProfit
			if bar.Last < exitPrice {
				continue
			}

			order, err := types.NewOrder(bar.Symbol,
				types.Sell,
				types.Limit,
				types.GTC,
				strat.PositionSize,
				exitPrice,
			)
			if err != nil {
				return orders
			}

			orders = append(orders, order)
		}
	}
	return orders
}
