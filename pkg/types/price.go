package types

import (
	"fmt"
	"math"
	"strconv"
	"strings"
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

func ParsePrice(s string) (Price, error) {
	// Split on '.' and extract whole and fractional part
	if (len(s) == 0) {
		return 0, fmt.Errorf("empty string")
	}

	isNegative := false
	if (s[0] == '-') {
		s = s[1:]
		isNegative = true
	}

	values := strings.Split(s, ".")

	if (len(values) > 2) {
		return 0, fmt.Errorf("malformed numeric string: %s", s)
	}

	var fracPart string
	wholePart := values[0]
	if (len(values) <= 1) {
		fracPart = ""
	} else {
		fracPart = values[1]
	}

	// Turn original value into an integer and convert to Price
	wholeInt, err := strconv.ParseInt(wholePart, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse whole part %q: %w", wholePart, err)
	}

	// For now, we naively drop any digits after the 4th
	if (len(fracPart) > 4) {
		fracPart = fracPart[:4]
	}

	fracInt, err := strconv.ParseInt(fracPart + strings.Repeat("0", 4-len(fracPart)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse fractional part %q: %w", fracPart, err)
	}

	res := wholeInt * int64(TicksPerUnit) + fracInt

	// Negate if original price was negative
	if isNegative {
		return Price(-res), nil
	}
	return Price(res), nil
}