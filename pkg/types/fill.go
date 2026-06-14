package types

import (
	"fmt"
	"time"
)

type Fill struct {
	Symbol      string
	Side        Side
	Type        OrderType
	Quantity    int64 
	Price       Price 
	SubmittedAt time.Time
}

func NewFill(symbol string, side Side, orderType OrderType, qty int64, price Price) (Fill, error) {

	fill := Fill{
		Symbol:      symbol,
		Side:        side,
		Type:        orderType,
		Quantity:    qty,
		Price:       price,
		SubmittedAt: time.Now(),
	}

	if err := fill.validate(); err != nil {
		return Fill{}, err
	}

	return fill, nil
}

func (o Fill) validate() error {
	if !o.Side.isValid() {
		return fmt.Errorf("invalid Side: %d", o.Side)
	}
	if !o.Type.isValid() {
		return fmt.Errorf("invalid OrderType: %d", o.Type)
	}
	return nil
}
