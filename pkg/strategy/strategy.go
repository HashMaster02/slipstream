package strategy

import (
	"github.com/HashMaster02/slipstream/internal/data"
	"github.com/HashMaster02/slipstream/internal/portfolio"
	"github.com/HashMaster02/slipstream/pkg/types"
)

type Strategy interface {
	CalculateSignals(marketData *map[string]*data.Row, port *portfolio.Portfolio) []types.Order
}
