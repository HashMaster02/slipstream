package strategy

import (
	"github.com/HashMaster02/slipstream/internal/data"
	"github.com/HashMaster02/slipstream/internal/portfolio"
	"github.com/HashMaster02/slipstream/pkg/types"
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

func (strat *TakeProfit) CalculateSignals(marketData *map[string]*data.Row, port *portfolio.Portfolio) []types.Order {

	orders := make([]types.Order, len(strat.Watchlist))

	for _, symbol := range strat.Watchlist {
		bar := (*marketData)[symbol]
		position, succ := port.Positions[bar.Symbol]

		if !succ {
			// Init a new position within the portfolio
			position = &portfolio.Position{
				Symbol: symbol,
				Qty: 0,
				CurrentSharePrice: types.PriceFromFloat(0),
				CostBasis: types.PriceFromFloat(0),
			}
			port.Positions[symbol] = position
		}

		if position.Qty == 0 {
			// Buy signal
			position.CurrentSharePrice = bar.Close

			order, err := types.NewOrder(bar.Symbol,
							types.Buy,
							types.Market,
							types.GTC,
							strat.PositionSize,
							bar.Close,
						)
			if err != nil {
				return orders
			}
			orders = append(orders, order)

		} else {
			// Sell signal
			exitPrice := position.CostBasis + strat.TakeProfit
			if bar.Close < exitPrice {
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
