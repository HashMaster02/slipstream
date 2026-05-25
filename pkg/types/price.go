package types

import (
	"fmt"
	"math"
)

// Price is a fixed-point monetary value in 1/10000ths of a unit (1 pip = 0.0001).
// Stored as int64 to avoid floating-point error.
// MaxPrice ≈ 9.2 × 10^14 / 10000 = 9.2 × 10^10
type Price int64

const TicksPerUnit Price = 10000

// PriceFromFloat converts a float64 into a Price.
func PriceFromFloat(f float64) Price {
	return Price(math.Round(f * float64(TicksPerUnit)))
}

// Float converts a Price into a float64.
func (p Price) Float() float64 {
	return float64(p) / float64(TicksPerUnit)
}

// String returns the string representation of a Price rounded to 4 decimal places.
func (p Price) String() string {
	return fmt.Sprintf("%0.4f", p.Float())
}
