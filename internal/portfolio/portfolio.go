package portfolio

import (
	"fmt"
	"strings"

	"github.com/HashMaster02/slipstream/internal/data"
	"github.com/HashMaster02/slipstream/internal/events"
	"github.com/HashMaster02/slipstream/pkg/types"
)

type Position struct {
	Symbol     string
	Qty        int64
	SharePrice types.Price
}

type Portfolio struct {
	Positions map[string]*Position
}

// TODO: This function should receive a FillEvent in the future
func (p *Portfolio) UpdatePosition(order events.OrderEvent) {

	position, ok := p.Positions[order.Symbol]
	if !ok {
		position = &Position{Symbol: order.Symbol, Qty: order.PositionSize, SharePrice: order.Price}
		p.Positions[order.Symbol] = position
		return
	}

	position.Qty += int64(order.OrderDirection) * order.PositionSize
	position.SharePrice = order.Price

}

func (p *Portfolio) UpdatePrice(bar data.Row) {

	position, ok := p.Positions[bar.Symbol]
	if !ok {
		return
	}

	position.SharePrice = bar.Close

}

func (p *Portfolio) String() string {
	var perfLog strings.Builder

	for ticker, pos := range p.Positions {
		if pos.Qty != 0 {
			fmt.Fprintf(&perfLog, "%s: %d shares at $%s/share\n", ticker, pos.Qty, pos.SharePrice.String())
		}
	}

	return perfLog.String()
}
