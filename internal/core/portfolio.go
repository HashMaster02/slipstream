package core

import (
	"github.com/HashMaster02/slipstream/internal/data"
	"github.com/HashMaster02/slipstream/pkg/types"
)

type Position struct {
	Symbol     string
	Qty        int64
	CurrentSharePrice types.Price
	CostBasis  types.Price
}

type Portfolio struct {
	Positions map[string]*Position
}

func (p *Portfolio) UpdatePosition(fill *types.Fill) {
	position, ok := p.Positions[fill.Symbol]
	if !ok {
		position = &Position{Symbol: fill.Symbol, Qty: int64(fill.Side) * fill.Quantity , CurrentSharePrice: fill.Price, CostBasis: fill.Price}
		p.Positions[fill.Symbol] = position
		return
	}

	position.Qty += int64(fill.Side) * fill.Quantity
	position.CostBasis= fill.Price
	position.CurrentSharePrice = fill.Price
}

func (p *Portfolio) UpdatePrice(bar data.Candle) {

	position, ok := p.Positions[bar.Symbol]
	if !ok {
		return
	}

	position.CurrentSharePrice = bar.Close

}
