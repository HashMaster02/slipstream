package strategy

import (
	"github.com/HashMaster02/slipstream/src/core"
	"github.com/HashMaster02/slipstream/src/data"
	"github.com/HashMaster02/slipstream/src/types"
)

type Strategy interface {
	CalculateSignals(marketData *map[string]*data.Candle, port *core.Portfolio) []types.Order
}
