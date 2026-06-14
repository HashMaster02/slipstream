package metrics

import (
	"fmt"
	"strings"

	"github.com/HashMaster02/slipstream/internal/core"
	"github.com/HashMaster02/slipstream/pkg/types"
)

func NetAssetValue(portfolio *core.Portfolio) string {
	var perfLog strings.Builder

	for ticker, pos := range portfolio.Positions {
		if pos.Qty != 0 {
			fmt.Fprintf(&perfLog, "\x1b[1;36m%5s:\x1b[0m %d shares at $%s/share\n", ticker, pos.Qty, pos.CurrentSharePrice.String())
		}
	}

	return perfLog.String()
}

func LargestPosition(portfolio *core.Portfolio) (string, types.Price) {
	type MaxPosition struct {
		MarketValue types.Price
		Symbol      string
	}

	var max MaxPosition = MaxPosition{MarketValue: 0, Symbol: ""}

	for ticker, pos := range portfolio.Positions {
		if pos.Qty != 0 {
			marketVal := pos.CurrentSharePrice * types.Price(pos.Qty)
			if max.MarketValue < marketVal {
				max.MarketValue = marketVal
				max.Symbol = ticker
			}
		}
	}

	return max.Symbol, max.MarketValue
}
