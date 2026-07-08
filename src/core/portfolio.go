package core

import (
	"github.com/HashMaster02/slipstream/src/data"
	"github.com/HashMaster02/slipstream/src/types"
)

type Position struct {
	Symbol            string
	Qty               int64
	CurrentSharePrice types.Price
	CostBasis         types.Price
}

type Portfolio struct {
	Positions map[string]*Position
	NAV       types.Price
}

func (p *Portfolio) UpdatePosition(fill *types.Fill) {
	position, ok := p.Positions[fill.Symbol]
	if !ok {
		position = &Position{Symbol: fill.Symbol, Qty: int64(fill.Side) * fill.Quantity, CurrentSharePrice: fill.Price, CostBasis: fill.Price}
		p.Positions[fill.Symbol] = position
		return
	}

	position.Qty += int64(fill.Side) * fill.Quantity
	position.CostBasis = fill.Price
	position.CurrentSharePrice = fill.Price
	p.updateNAV()
}

func (p *Portfolio) UpdatePrice(bar data.Quote) {

	position, ok := p.Positions[bar.Symbol]
	if !ok {
		return
	}

	position.CurrentSharePrice = bar.Last
	p.updateNAV()

}

// TODO: This logic can be optimized. Avoid recomputing on every tick
func (p *Portfolio) updateNAV() {
	var newNav types.Price = types.PriceFromFloat(0.0)
	for _, pos := range p.Positions {
		newNav += types.Price(pos.Qty) * pos.CurrentSharePrice
	}
	p.NAV = newNav
}

