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
	Cash      types.Price
	NAV       types.Price
}

func (p *Portfolio) CanAfford(qty int64, price types.Price) bool {
	return types.Price(qty)*price <= p.Cash
}

func (p *Portfolio) UpdatePosition(fill *types.Fill) {
	p.Cash -= types.Price(int64(fill.Side)*fill.Quantity) * fill.Price

	position, ok := p.Positions[fill.Symbol]
	if !ok {
		position = &Position{Symbol: fill.Symbol, Qty: int64(fill.Side) * fill.Quantity, CurrentSharePrice: fill.Price, CostBasis: fill.Price}
		p.Positions[fill.Symbol] = position
		p.updateNAV()
		return
	}

	signed := int64(fill.Side) * fill.Quantity
	newQty := position.Qty + signed

	// Only trades that grow the position move the cost basis. Reducing it leaves
	// the basis of the remaining shares alone.
	if position.Qty == 0 {
		position.CostBasis = fill.Price
	} else if (position.Qty > 0) == (signed > 0) {
		oldCost := types.Price(abs(position.Qty)) * position.CostBasis
		addCost := types.Price(abs(signed)) * fill.Price
		position.CostBasis = (oldCost + addCost) / types.Price(abs(newQty))
	}

	position.Qty = newQty
	position.CurrentSharePrice = fill.Price
	p.updateNAV()
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
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
	newNav := p.Cash
	for _, pos := range p.Positions {
		newNav += types.Price(pos.Qty) * pos.CurrentSharePrice
	}
	p.NAV = newNav
}

