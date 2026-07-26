package main

import (
	"container/list"
	"testing"
	"time"

	"github.com/HashMaster02/slipstream/src/core"
	"github.com/HashMaster02/slipstream/src/data"
	"github.com/HashMaster02/slipstream/src/types"
)

// ---- tiny builders to keep the table readable -----------------------------

// px is a shorthand for a fixed-point Price from a float dollar value.
func px(f float64) types.Price { return types.PriceFromFloat(f) }

// quote builds the minimal Quote ProcessOrders reads: buys match and fill
// against Ask, sells against Bid. Last is unused by order matching.
func quote(sym string, bid, ask float64) *data.Quote {
	return &data.Quote{Symbol: sym, Bid: px(bid), Ask: px(ask)}
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

// setPending replaces the package-level book with a fresh list holding the
// given orders, in order. pendingOrders is a container/list.List of
// *types.Order, matching what the engine pushes in main.
func setPending(orders ...*types.Order) {
	pendingOrders = list.New()
	for _, o := range orders {
		pendingOrders.PushBack(o)
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
	if p.CurrentSharePrice != px(price) {
		t.Errorf("%s fill price = %s, want %s", sym, p.CurrentSharePrice, px(price))
	}
}

// wantPendingIDs asserts exactly which order IDs are still resting, in order.
// pendingOrders is a doubly linked list of *types.Order.
func wantPendingIDs(t *testing.T, ids ...uint64) {
	t.Helper()
	if pendingOrders.Len() != len(ids) {
		t.Fatalf("have %d pending orders, want %d (ids %v)", pendingOrders.Len(), len(ids), ids)
	}
	i := 0
	for e := pendingOrders.Front(); e != nil; e = e.Next() {
		o, ok := e.Value.(*types.Order)
		if !ok {
			t.Fatalf("pending[%d] is not a *types.Order (got %T)", i, e.Value)
		}
		if uint64(o.ID) != ids[i] {
			t.Errorf("pending[%d].ID = %d, want %d", i, o.ID, ids[i])
		}
		i++
	}
}

// TestProcessOrders pins down the order-matching contract: given the bars the
// engine has seen and the orders resting in the book, which orders fill, at
// what price, and which orders remain resting afterwards.
//
// Each case fully owns the package-level engine state (latestQuote /
// pendingOrders) and returns the portfolio to inspect.
func TestProcessOrders(t *testing.T) {
	tests := []struct {
		name  string
		setup func() *core.Portfolio
		check func(t *testing.T, port *core.Portfolio)
	}{
		{
			name: "market buy fills at the ask",
			setup: func() *core.Portfolio {
				latestQuote = map[string]*data.Quote{"AAPL": quote("AAPL", 149, 150)}
				setPending(ord(1, "AAPL", types.Buy, types.Market, 100, 0))
				return &core.Portfolio{Positions: map[string]*core.Position{}}
			},
			check: func(t *testing.T, port *core.Portfolio) {
				wantPosition(t, port, "AAPL", 100)
				wantFillPrice(t, port, "AAPL", 150) // ask, not bid
				wantPendingIDs(t)                   // consumed
			},
		},
		{
			name: "limit buy fills when ask is at/below the limit",
			setup: func() *core.Portfolio {
				latestQuote = map[string]*data.Quote{"AAPL": quote("AAPL", 94, 95)}
				setPending(ord(1, "AAPL", types.Buy, types.Limit, 100, 100))
				return &core.Portfolio{Positions: map[string]*core.Position{}}
			},
			check: func(t *testing.T, port *core.Portfolio) {
				wantPosition(t, port, "AAPL", 100)
				wantFillPrice(t, port, "AAPL", 95) // fills at ask
				wantPendingIDs(t)
			},
		},
		{
			name: "limit buy rests when ask is above the limit",
			setup: func() *core.Portfolio {
				latestQuote = map[string]*data.Quote{"AAPL": quote("AAPL", 104, 105)}
				setPending(ord(1, "AAPL", types.Buy, types.Limit, 100, 100))
				return &core.Portfolio{Positions: map[string]*core.Position{}}
			},
			check: func(t *testing.T, port *core.Portfolio) {
				if _, ok := port.Positions["AAPL"]; ok {
					t.Errorf("AAPL should not have a position; order should not have filled")
				}
				wantPendingIDs(t, 1) // still resting
			},
		},
		{
			name: "limit sell fills when bid is at/above the limit",
			setup: func() *core.Portfolio {
				latestQuote = map[string]*data.Quote{"AAPL": quote("AAPL", 115, 116)}
				setPending(ord(1, "AAPL", types.Sell, types.Limit, 100, 110))
				return &core.Portfolio{Positions: map[string]*core.Position{
					"AAPL": pos("AAPL", 100, 100),
				}}
			},
			check: func(t *testing.T, port *core.Portfolio) {
				wantPosition(t, port, "AAPL", 0)    // sold out
				wantFillPrice(t, port, "AAPL", 115) // fills at bid
				wantPendingIDs(t)
			},
		},
		{
			name: "limit sell rests when bid is below the limit",
			setup: func() *core.Portfolio {
				latestQuote = map[string]*data.Quote{"AAPL": quote("AAPL", 105, 106)}
				setPending(ord(1, "AAPL", types.Sell, types.Limit, 100, 110))
				return &core.Portfolio{Positions: map[string]*core.Position{
					"AAPL": pos("AAPL", 100, 100),
				}}
			},
			check: func(t *testing.T, port *core.Portfolio) {
				wantPosition(t, port, "AAPL", 100) // unchanged
				wantPendingIDs(t, 1)
			},
		},
		{
			name: "order with no market data yet stays resting",
			setup: func() *core.Portfolio {
				latestQuote = map[string]*data.Quote{} // no quote for AAPL
				setPending(ord(1, "AAPL", types.Buy, types.Market, 100, 0))
				return &core.Portfolio{Positions: map[string]*core.Position{}}
			},
			check: func(t *testing.T, port *core.Portfolio) {
				if _, ok := port.Positions["AAPL"]; ok {
					t.Errorf("AAPL should not have a position without market data")
				}
				wantPendingIDs(t, 1)
			},
		},
		{
			name: "multiple fillable orders on one symbol both execute (scale-in)",
			setup: func() *core.Portfolio {
				latestQuote = map[string]*data.Quote{"AAPL": quote("AAPL", 94, 95)}
				setPending(
					ord(1, "AAPL", types.Buy, types.Limit, 100, 100), // ask 95<=100 fills
					ord(2, "AAPL", types.Buy, types.Limit, 100, 96),  // ask 95<=96  fills
				)
				return &core.Portfolio{Positions: map[string]*core.Position{}}
			},
			check: func(t *testing.T, port *core.Portfolio) {
				wantPosition(t, port, "AAPL", 200)
				wantPendingIDs(t)
			},
		},
		{
			name: "fillable order behind a resting order removes the right one",
			setup: func() *core.Portfolio {
				latestQuote = map[string]*data.Quote{"AAPL": quote("AAPL", 94, 95)}
				setPending(
					ord(1, "AAPL", types.Buy, types.Limit, 100, 50), // ask 95<=50 false -> should rest
					ord(2, "AAPL", types.Buy, types.Market, 100, 0), // fills
				)
				return &core.Portfolio{Positions: map[string]*core.Position{}}
			},
			check: func(t *testing.T, port *core.Portfolio) {
				wantPosition(t, port, "AAPL", 100)
				// The market order (id 2) filled and must be removed; the
				// unfillable limit order (id 1) must still be resting.
				wantPendingIDs(t, 1)
			},
		},
		{
			name: "filled order is consumed but a resting sibling survives",
			setup: func() *core.Portfolio {
				latestQuote = map[string]*data.Quote{"AAPL": quote("AAPL", 94, 95)}
				setPending(
					ord(1, "AAPL", types.Buy, types.Market, 100, 0), // fills
					ord(2, "AAPL", types.Buy, types.Limit, 100, 50), // ask 95<=50 false -> should rest
				)
				return &core.Portfolio{Positions: map[string]*core.Position{}}
			},
			check: func(t *testing.T, port *core.Portfolio) {
				wantPosition(t, port, "AAPL", 100)
				// The market order filled and should be gone, but the
				// unfillable limit order (id 2) must still be resting.
				wantPendingIDs(t, 2)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			port := tc.setup()
			ProcessOrders(port, time.Time{})
			tc.check(t, port)
		})
	}
}

// TestProcessOrdersStaleQuote pins that a symbol whose feed has ended stops
// filling orders once its last quote ages past maxQuoteStaleness.
func TestProcessOrdersStaleQuote(t *testing.T) {
	maxQuoteStaleness = 60 * time.Minute
	staleSymbols = map[string]bool{}
	defer func() { maxQuoteStaleness = 0 }()

	lastQuote := time.Date(2026, 4, 30, 16, 22, 0, 0, time.UTC)
	bar := quote("DTSQR", 0.149, 0.150)
	bar.Timestamp = lastQuote
	latestQuote = map[string]*data.Quote{"DTSQR": bar}

	setPending(ord(1, "DTSQR", types.Buy, types.Market, 100, 0))
	port := &core.Portfolio{Positions: map[string]*core.Position{}}

	ProcessOrders(port, lastQuote.Add(30*time.Minute))
	wantPosition(t, port, "DTSQR", 100)

	setPending(ord(2, "DTSQR", types.Buy, types.Market, 100, 0))
	ProcessOrders(port, lastQuote.Add(26*24*time.Hour))

	if port.Positions["DTSQR"].Qty != 100 {
		t.Errorf("DTSQR qty = %d, want 100; order filled against a stale quote", port.Positions["DTSQR"].Qty)
	}
	wantPendingIDs(t, 2)
	if !staleSymbols["DTSQR"] {
		t.Errorf("DTSQR was not recorded as stale")
	}
}
