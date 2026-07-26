package types

import (
	"fmt"
	"sync/atomic"
	"time"
)

// OrderID is a simple uint64.
type OrderID uint64

var nextOrderID atomic.Uint64

func NewOrderID() OrderID {
	return OrderID(nextOrderID.Add(1))
}

// Side can be either Buy or Sell
type Side int8

const (
	Buy  Side = 1
	Sell Side = -1
)

func (s Side) String() string {
	switch s {
	case Buy:
		return "buy"
	case Sell:
		return "sell"
	default:
		return fmt.Sprintf("Side(%d)", int8(s))

	}
}

func (s Side) isValid() bool {
	return s == Buy || s == Sell
}

// OrderType can be Limit or Market
type OrderType int8

const (
	Limit OrderType = iota
	Market
)

func (ot OrderType) String() string {
	switch ot {
	case Limit:
		return "LIMIT"
	case Market:
		return "MARKET"
	default:
		return fmt.Sprintf("OrderType(%d)", int8(ot))
	}
}

func (ot OrderType) isValid() bool {
	return ot == Limit || ot == Market
}

// TIF is the 'time in force' which determines when an order will expire. Can be Day, IOC, or GTC
type TIF int8

const (
	Day TIF = iota
	IOC
	GTC
)

func (t TIF) String() string {
	switch t {
	case Day:
		return "Day"
	case IOC:
		return "IOC"
	case GTC:
		return "GTC"
	default:
		return fmt.Sprintf("TIF(%d)", int8(t))
	}
}

func (t TIF) isValid() bool {
	return t == Day || t == IOC || t == GTC
}

// Order is an object with all the details for a trade order. This should always be constructed using NewOrder().
type Order struct {
	ID          OrderID
	Symbol      string
	Side        Side
	Type        OrderType
	TIF         TIF
	Quantity    int64 // shares
	Price       Price // limit price (0 if Market)
	SubmittedAt time.Time
}

func NewOrder(symbol string, side Side, orderType OrderType, tif TIF, qty int64, price Price, submittedAt time.Time) (Order, error) {

	order := Order{
		ID:          NewOrderID(),
		Symbol:      symbol,
		Side:        side,
		Type:        orderType,
		TIF:         tif,
		Quantity:    qty,
		Price:       price,
		SubmittedAt: submittedAt,
	}

	if err := order.validate(); err != nil {
		return Order{}, err
	}

	return order, nil
}

func (o Order) validate() error {
	if !o.Side.isValid() {
		return fmt.Errorf("invalid Side: %d", o.Side)
	}
	if !o.Type.isValid() {
		return fmt.Errorf("invalid OrderType: %d", o.Type)
	}
	if !o.TIF.isValid() {
		return fmt.Errorf("invalid TIF: %d", o.TIF)
	}
	return nil
}

func (o Order) String() string {
	return fmt.Sprintf("Order ID: %d\nSymbol: %s\nSide: %s\nType: %s\nTIF: %s\nQty: %d\nPrice: %s\ntimestamp: %s",
		o.ID, o.Symbol, o.Side.String(), o.Type.String(), o.TIF.String(), o.Quantity, o.Price.String(), o.SubmittedAt)
}
