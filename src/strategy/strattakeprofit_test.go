package strategy

import (
	"testing"

	"github.com/HashMaster02/slipstream/src/core"
	"github.com/HashMaster02/slipstream/src/data"
	"github.com/HashMaster02/slipstream/src/types"
)

func px(f float64) types.Price { return types.PriceFromFloat(f) }

func quote(sym string, last float64) *data.Quote {
	return &data.Quote{Symbol: sym, Bid: px(last - 0.01), Ask: px(last + 0.01), Last: px(last)}
}

// tick runs one CalculateSignals pass and asserts how many orders came out.
func tick(t *testing.T, strat *TakeProfit, md map[string]*data.Quote, port *core.Portfolio, wantOrders int, msg string) []types.Order {
	t.Helper()
	orders := strat.CalculateSignals(&md, port)
	if len(orders) != wantOrders {
		t.Fatalf("%s: got %d orders, want %d", msg, len(orders), wantOrders)
	}
	return orders
}

// TestTakeProfitLifecycle walks one symbol through the full trade lifecycle
// and asserts the strategy emits exactly one order per transition, staying
// silent while an order is in flight (the simulated latency window).
func TestTakeProfitLifecycle(t *testing.T) {
	strat := NewTakeProfit([]string{"AAPL"}, px(0.50), 100)
	port := &core.Portfolio{Positions: map[string]*core.Position{}}
	md := map[string]*data.Quote{}

	// No quote yet: nothing to do, and critically, no crash.
	tick(t, &strat, md, port, 0, "no quote yet")

	// First quote arrives: exactly one market buy.
	md["AAPL"] = quote("AAPL", 100)
	orders := tick(t, &strat, md, port, 1, "first quote")
	if orders[0].Side != types.Buy || orders[0].Type != types.Market {
		t.Fatalf("expected market buy, got %s %s", orders[0].Side, orders[0].Type)
	}
	if orders[0].Quantity != 100 {
		t.Fatalf("buy qty = %d, want 100", orders[0].Quantity)
	}

	// Latency window: buy is in flight, position still empty. No duplicates.
	tick(t, &strat, md, port, 0, "buy in flight (tick 1)")
	tick(t, &strat, md, port, 0, "buy in flight (tick 2)")

	// Fill lands.
	port.Positions["AAPL"] = &core.Position{Symbol: "AAPL", Qty: 100, CurrentSharePrice: px(100), CostBasis: px(100)}

	// Long, but below the exit target: hold.
	tick(t, &strat, md, port, 0, "fill observed, below target")
	md["AAPL"] = quote("AAPL", 100.25)
	tick(t, &strat, md, port, 0, "still below target")

	// Last has reached the target but the bid has not: there is nothing to sell
	// into yet, so the strategy holds.
	md["AAPL"] = quote("AAPL", 100.50)
	tick(t, &strat, md, port, 0, "last at target, bid below")

	// Bid reaches the target: exactly one market sell for the full position.
	md["AAPL"] = quote("AAPL", 100.60)
	orders = tick(t, &strat, md, port, 1, "target hit")
	if orders[0].Side != types.Sell || orders[0].Type != types.Market {
		t.Fatalf("expected market sell, got %s %s", orders[0].Side, orders[0].Type)
	}
	if orders[0].Quantity != 100 {
		t.Fatalf("sell qty = %d, want 100", orders[0].Quantity)
	}

	// Latency window again: sell in flight, price still above target. No duplicates.
	md["AAPL"] = quote("AAPL", 101)
	tick(t, &strat, md, port, 0, "sell in flight (tick 1)")
	tick(t, &strat, md, port, 0, "sell in flight (tick 2)")

	// Sell fills; one tick to observe it and go flat, then re-enter.
	port.Positions["AAPL"].Qty = 0
	tick(t, &strat, md, port, 0, "sell fill observed, back to flat")
	orders = tick(t, &strat, md, port, 1, "re-entry")
	if orders[0].Side != types.Buy {
		t.Fatalf("expected re-entry buy, got %s", orders[0].Side)
	}
}

// TestTakeProfitSellsActualHoldings pins that the exit order is sized to the
// position, not the configured entry size.
func TestTakeProfitSellsActualHoldings(t *testing.T) {
	strat := NewTakeProfit([]string{"AAPL"}, px(0.50), 100)
	md := map[string]*data.Quote{"AAPL": quote("AAPL", 100.60)}
	port := &core.Portfolio{Positions: map[string]*core.Position{
		"AAPL": {Symbol: "AAPL", Qty: 250, CurrentSharePrice: px(100.60), CostBasis: px(100)},
	}}

	// Force the state machine to Long via its own transitions: Flat emits a
	// buy (state -> Entering), then the existing position moves it to Long.
	tick(t, &strat, md, port, 1, "flat emits entry")
	tick(t, &strat, md, port, 0, "entering observes position")

	orders := tick(t, &strat, md, port, 1, "long emits exit")
	if orders[0].Quantity != 250 {
		t.Fatalf("sell qty = %d, want the held 250", orders[0].Quantity)
	}
}

// TestTakeProfitMissingQuoteDoesNotPanic covers the multi-ticker case where
// only some symbols have market data yet (audit bug C1).
func TestTakeProfitMissingQuoteDoesNotPanic(t *testing.T) {
	strat := NewTakeProfit([]string{"AAPL", "MSFT"}, px(0.50), 100)
	md := map[string]*data.Quote{"AAPL": quote("AAPL", 100)} // no MSFT quote
	port := &core.Portfolio{Positions: map[string]*core.Position{}}

	orders := tick(t, &strat, md, port, 1, "one quoted symbol")
	if orders[0].Symbol != "AAPL" {
		t.Fatalf("order symbol = %s, want AAPL", orders[0].Symbol)
	}
}

// TestTakeProfitStrategyDoesNotMutatePortfolio pins that signal calculation
// is read-only on the portfolio (audit issue H5).
func TestTakeProfitStrategyDoesNotMutatePortfolio(t *testing.T) {
	strat := NewTakeProfit([]string{"AAPL"}, px(0.50), 100)
	md := map[string]*data.Quote{"AAPL": quote("AAPL", 100)}
	port := &core.Portfolio{Positions: map[string]*core.Position{}}

	tick(t, &strat, md, port, 1, "entry")
	if len(port.Positions) != 0 {
		t.Fatalf("strategy created %d position(s) in the portfolio; want 0", len(port.Positions))
	}
}
