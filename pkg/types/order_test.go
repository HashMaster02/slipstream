package types_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/HashMaster02/slipstream/pkg/types"
)

func TestSide_String(t *testing.T) {
	tests := []struct {
		name string
		in   types.Side
		want string
	}{
		{"buy", types.Buy, "buy"},
		{"sell", types.Sell, "sell"},
		{"zero", types.Side(0), "Side(0)"},
		{"unknown", types.Side(42), "Side(42)"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in.String()
			if got != tc.want {
				t.Errorf("Side(%d).String() = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestOrderType_String(t *testing.T) {
	tests := []struct {
		name string
		in   types.OrderType
		want string
	}{
		{"limit", types.Limit, "LIMIT"},
		{"market", types.Market, "MARKET"},
		{"unknown positive", types.OrderType(7), "OrderType(7)"},
		{"negative", types.OrderType(-1), "OrderType(-1)"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in.String()
			if got != tc.want {
				t.Errorf("OrderType(%d).String() = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestTIF_String(t *testing.T) {
	tests := []struct {
		name string
		in   types.TIF
		want string
	}{
		{"day", types.Day, "Day"},
		{"ioc", types.IOC, "IOC"},
		{"gtc", types.GTC, "GTC"},
		{"unknown positive", types.TIF(9), "TIF(9)"},
		{"negative", types.TIF(-2), "TIF(-2)"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in.String()
			if got != tc.want {
				t.Errorf("TIF(%d).String() = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNewOrder_PopulatesFields(t *testing.T) {
	before := time.Now()
	o, err := types.NewOrder("AAPL", types.Buy, types.Limit, types.GTC, 100, types.PriceFromFloat(123.4567))
	after := time.Now()
	if err != nil {
		t.Fatalf("NewOrder returned unexpected error: %v", err)
	}

	if o.ID == 0 {
		t.Errorf("expected non-zero ID, got %d", o.ID)
	}
	if o.Symbol != "AAPL" {
		t.Errorf("Symbol = %q, want %q", o.Symbol, "AAPL")
	}
	if o.Side != types.Buy {
		t.Errorf("Side = %v, want Buy", o.Side)
	}
	if o.Type != types.Limit {
		t.Errorf("Type = %v, want Limit", o.Type)
	}
	if o.TIF != types.GTC {
		t.Errorf("TIF = %v, want GTC", o.TIF)
	}
	if o.Quantity != 100 {
		t.Errorf("Quantity = %d, want 100", o.Quantity)
	}
	if o.Price != types.PriceFromFloat(123.4567) {
		t.Errorf("Price = %v, want 123.4567", o.Price)
	}
	if o.SubmittedAt.Before(before) || o.SubmittedAt.After(after) {
		t.Errorf("SubmittedAt %v not in [%v, %v]", o.SubmittedAt, before, after)
	}
}

func TestNewOrder_AssignsMonotonicIDs(t *testing.T) {
	o1, err := types.NewOrder("AAPL", types.Buy, types.Limit, types.GTC, 1, 0)
	if err != nil {
		t.Fatalf("NewOrder failed: %v", err)
	}
	o2, err := types.NewOrder("AAPL", types.Buy, types.Limit, types.GTC, 1, 0)
	if err != nil {
		t.Fatalf("NewOrder failed: %v", err)
	}
	if o2.ID <= o1.ID {
		t.Errorf("expected o2.ID > o1.ID, got o1=%d o2=%d", o1.ID, o2.ID)
	}
}

func TestNewOrder_InvalidInputs(t *testing.T) {
	tests := []struct {
		name      string
		side      types.Side
		orderType types.OrderType
		tif       types.TIF
		wantErr   string
	}{
		{"side zero", types.Side(0), types.Limit, types.GTC, "invalid Side"},
		{"side unknown", types.Side(42), types.Limit, types.GTC, "invalid Side"},
		{"type unknown", types.Buy, types.OrderType(7), types.GTC, "invalid OrderType"},
		{"type negative", types.Buy, types.OrderType(-1), types.GTC, "invalid OrderType"},
		{"tif unknown", types.Buy, types.Limit, types.TIF(9), "invalid TIF"},
		{"tif negative", types.Buy, types.Limit, types.TIF(-2), "invalid TIF"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := types.NewOrder("AAPL", tc.side, tc.orderType, tc.tif, 1, 0)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestNewOrderID_Unique(t *testing.T) {
	const n = 1000
	seen := make(map[types.OrderID]struct{}, n)
	var prev types.OrderID
	for i := range n {
		id := types.NewOrderID()
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate ID %d at iteration %d", id, i)
		}
		seen[id] = struct{}{}
		if i > 0 && id <= prev {
			t.Errorf("expected monotonic IDs, got %d after %d", id, prev)
		}
		prev = id
	}
}

func TestOrder_String_LimitBuyGTC(t *testing.T) {
	o, err := types.NewOrder("AAPL", types.Buy, types.Limit, types.GTC, 100, types.PriceFromFloat(123.4567))
	if err != nil {
		t.Fatalf("NewOrder failed: %v", err)
	}

	got := o.String()
	wants := []string{
		fmt.Sprintf("Order ID: %d", o.ID),
		"Symbol: AAPL",
		"Side: buy",
		"Type: LIMIT",
		"TIF: GTC",
		"Qty: 100",
		"Price: 123.4567",
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("Order.String() missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestOrder_String_MarketSellIOC(t *testing.T) {
	o, err := types.NewOrder("TSLA", types.Sell, types.Market, types.IOC, 50, 0)
	if err != nil {
		t.Fatalf("NewOrder failed: %v", err)
	}

	got := o.String()
	wants := []string{
		"Symbol: TSLA",
		"Side: sell",
		"Type: MARKET",
		"TIF: IOC",
		"Qty: 50",
		"Price: 0.0000",
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("Order.String() missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestOrder_String_NegativeQuantityAndPrice(t *testing.T) {
	o, err := types.NewOrder("MSFT", types.Sell, types.Limit, types.Day, -10, types.PriceFromFloat(-5.5))
	if err != nil {
		t.Fatalf("NewOrder failed: %v", err)
	}

	got := o.String()
	if !strings.Contains(got, "Qty: -10") {
		t.Errorf("Order.String() missing negative quantity\nfull output:\n%s", got)
	}
	if !strings.Contains(got, "Price: -5.5000") {
		t.Errorf("Order.String() missing negative price\nfull output:\n%s", got)
	}
}
