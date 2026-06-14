package main

import (
	"testing"

	"github.com/HashMaster02/slipstream/internal/core"
	"github.com/HashMaster02/slipstream/internal/data"
	"github.com/HashMaster02/slipstream/pkg/types"
)

// ---- tiny builders to keep the table readable -----------------------------

// px is a shorthand for a fixed-point Price from a float dollar value.
func px(f float64) types.Price { return types.PriceFromFloat(f) }

// bar builds the minimal Candle ProcessOrders reads (it only looks at Close).
func bar(sym string, close float64) *data.Candle {
	return &data.Candle{Symbol: sym, Close: px(close)}
}

// ord builds an order with an explicit ID so we can identify which orders
// survive in pendingOrders after matching.
func ord(id uint64, sym string, side types.Side, ot types.OrderType, qty int64, price float64) *types.Order {
	return &types.Order{
		ID:       types.OrderID(id),
		Symbol:   sym,
		Side:     side,
		Type:     ot,
		TIF:      types.GTC,
		Quantity: qty,
		Price:    px(price),
	}
}

func pos(sym string, qty int64, cost float64) *core.Position {
	return &core.Position{
		Symbol:            sym,
		Qty:               qty,
		CurrentSharePrice: px(cost),
		CostBasis:         px(cost),
	}
}

// ---- assertions ------------------------------------------------------------

// wantPosition asserts a position exists with the expected quantity.
func wantPosition(t *testing.T, port *core.Portfolio, sym string, qty int64) {
	t.Helper()
	p, ok := port.Positions[sym]
	if !ok {
		t.Fatalf("expected a position for %s, but none exists", sym)
	}
	if p.Qty != qty {
		t.Errorf("%s qty = %d, want %d", sym, p.Qty, qty)
	}
}

// wantFillPrice asserts the most recent fill price recorded on the position.
func wantFillPrice(t *testing.T, port *core.Portfolio, sym string, price float64) {
	t.Helper()
	p, ok := port.Positions[sym]
	if !ok {
		t.Fatalf("expected a position for %s, but none exists", sym)
	}
	if p.CostBasis != px(price) {
		t.Errorf("%s fill price = %s, want %s", sym, p.CostBasis, px(price))
	}
}

// wantPendingIDs asserts exactly which order IDs are still resting for a symbol.
func wantPendingIDs(t *testing.T, sym string, ids ...uint64) {
	t.Helper()
	got := pendingOrders[sym]
	if len(got) != len(ids) {
		t.Fatalf("%s has %d pending orders, want %d (ids %v)", sym, len(got), len(ids), ids)
	}
	for i, o := range got {
		if uint64(o.ID) != ids[i] {
			t.Errorf("%s pending[%d].ID = %d, want %d", sym, i, o.ID, ids[i])
		}
	}
}

// TestProcessOrders pins down the order-matching contract: given the bars the
// engine has seen and the orders resting in the book, which orders fill, at
// what price, and which orders remain resting afterwards.
//
// Each case fully owns the package-level engine state (latestBar /
// pendingOrders) and returns the portfolio to inspect.
func TestProcessOrders(t *testing.T) {
	tests := []struct {
		name  string
		setup func() *core.Portfolio
		check func(t *testing.T, port *core.Portfolio)
	}{
		{
			name: "market buy fills at the bar close",
			setup: func() *core.Portfolio {
				latestBar = map[string]*data.Candle{"AAPL": bar("AAPL", 150)}
				pendingOrders = map[string][]*types.Order{
					"AAPL": {ord(1, "AAPL", types.Buy, types.Market, 100, 0)},
				}
				return &core.Portfolio{Positions: map[string]*core.Position{}}
			},
			check: func(t *testing.T, port *core.Portfolio) {
				wantPosition(t, port, "AAPL", 100)
				wantFillPrice(t, port, "AAPL", 150)
				wantPendingIDs(t, "AAPL") // consumed
			},
		},
		{
			name: "limit buy fills when close is at/below the limit",
			setup: func() *core.Portfolio {
				latestBar = map[string]*data.Candle{"AAPL": bar("AAPL", 95)}
				pendingOrders = map[string][]*types.Order{
					"AAPL": {ord(1, "AAPL", types.Buy, types.Limit, 100, 100)},
				}
				return &core.Portfolio{Positions: map[string]*core.Position{}}
			},
			check: func(t *testing.T, port *core.Portfolio) {
				wantPosition(t, port, "AAPL", 100)
				wantFillPrice(t, port, "AAPL", 95)
				wantPendingIDs(t, "AAPL")
			},
		},
		{
			name: "limit buy rests when close is above the limit",
			setup: func() *core.Portfolio {
				latestBar = map[string]*data.Candle{"AAPL": bar("AAPL", 105)}
				pendingOrders = map[string][]*types.Order{
					"AAPL": {ord(1, "AAPL", types.Buy, types.Limit, 100, 100)},
				}
				return &core.Portfolio{Positions: map[string]*core.Position{}}
			},
			check: func(t *testing.T, port *core.Portfolio) {
				if _, ok := port.Positions["AAPL"]; ok {
					t.Errorf("AAPL should not have a position; order should not have filled")
				}
				wantPendingIDs(t, "AAPL", 1) // still resting
			},
		},
		{
			name: "limit sell fills when close is at/above the limit",
			setup: func() *core.Portfolio {
				latestBar = map[string]*data.Candle{"AAPL": bar("AAPL", 115)}
				pendingOrders = map[string][]*types.Order{
					"AAPL": {ord(1, "AAPL", types.Sell, types.Limit, 100, 110)},
				}
				return &core.Portfolio{Positions: map[string]*core.Position{
					"AAPL": pos("AAPL", 100, 100),
				}}
			},
			check: func(t *testing.T, port *core.Portfolio) {
				wantPosition(t, port, "AAPL", 0) // sold out
				wantPendingIDs(t, "AAPL")
			},
		},
		{
			name: "limit sell rests when close is below the limit",
			setup: func() *core.Portfolio {
				latestBar = map[string]*data.Candle{"AAPL": bar("AAPL", 105)}
				pendingOrders = map[string][]*types.Order{
					"AAPL": {ord(1, "AAPL", types.Sell, types.Limit, 100, 110)},
				}
				return &core.Portfolio{Positions: map[string]*core.Position{
					"AAPL": pos("AAPL", 100, 100),
				}}
			},
			check: func(t *testing.T, port *core.Portfolio) {
				wantPosition(t, port, "AAPL", 100) // unchanged
				wantPendingIDs(t, "AAPL", 1)
			},
		},
		{
			name: "order with no market data yet stays resting",
			setup: func() *core.Portfolio {
				latestBar = map[string]*data.Candle{} // no bar for AAPL
				pendingOrders = map[string][]*types.Order{
					"AAPL": {ord(1, "AAPL", types.Buy, types.Market, 100, 0)},
				}
				return &core.Portfolio{Positions: map[string]*core.Position{}}
			},
			check: func(t *testing.T, port *core.Portfolio) {
				if _, ok := port.Positions["AAPL"]; ok {
					t.Errorf("AAPL should not have a position without market data")
				}
				wantPendingIDs(t, "AAPL", 1)
			},
		},
		{
			name: "multiple fillable orders on one symbol both execute (scale-in)",
			setup: func() *core.Portfolio {
				latestBar = map[string]*data.Candle{"AAPL": bar("AAPL", 95)}
				pendingOrders = map[string][]*types.Order{
					"AAPL": {
						ord(1, "AAPL", types.Buy, types.Limit, 100, 100), // 95<=100 fills
						ord(2, "AAPL", types.Buy, types.Limit, 100, 96),  // 95<=96  fills
					},
				}
				return &core.Portfolio{Positions: map[string]*core.Position{}}
			},
			check: func(t *testing.T, port *core.Portfolio) {
				wantPosition(t, port, "AAPL", 200)
				wantPendingIDs(t, "AAPL")
			},
		},
		{
			name: "fillable order behind a resting order removes the right one",
			setup: func() *core.Portfolio {
				latestBar = map[string]*data.Candle{"AAPL": bar("AAPL", 95)}
				pendingOrders = map[string][]*types.Order{
					"AAPL": {
						ord(1, "AAPL", types.Buy, types.Limit, 100, 50), // 95<=50 false -> should rest
						ord(2, "AAPL", types.Buy, types.Market, 100, 0),  // fills
					},
				}
				return &core.Portfolio{Positions: map[string]*core.Position{}}
			},
			check: func(t *testing.T, port *core.Portfolio) {
				wantPosition(t, port, "AAPL", 100)
				// The market order (id 2) filled and must be removed; the
				// unfillable limit order (id 1) must still be resting.
				wantPendingIDs(t, "AAPL", 1)
			},
		},
		{
			name: "filled order is consumed but a resting sibling survives",
			setup: func() *core.Portfolio {
				latestBar = map[string]*data.Candle{"AAPL": bar("AAPL", 95)}
				pendingOrders = map[string][]*types.Order{
					"AAPL": {
						ord(1, "AAPL", types.Buy, types.Market, 100, 0), // fills
						ord(2, "AAPL", types.Buy, types.Limit, 100, 50), // 95<=50 false -> should rest
					},
				}
				return &core.Portfolio{Positions: map[string]*core.Position{}}
			},
			check: func(t *testing.T, port *core.Portfolio) {
				wantPosition(t, port, "AAPL", 100)
				// The market order filled and should be gone, but the
				// unfillable limit order (id 2) must still be resting.
				wantPendingIDs(t, "AAPL", 2)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			port := tc.setup()
			ProcessOrders(port)
			tc.check(t, port)
		})
	}
}
