package strategy

import (
	"github.com/HashMaster02/slipstream/internal/core"
	"github.com/HashMaster02/slipstream/internal/data"
	"github.com/HashMaster02/slipstream/pkg/types"
)

type Strategy interface {
	CalculateSignals(marketData *map[string]*data.Quote, port *core.Portfolio) []types.Order
}
