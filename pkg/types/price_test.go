package types_test

import (
	"testing"

	"github.com/HashMaster02/slipstream/pkg/types"
)

func TestPriceFromFloat(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want types.Price
	}{
		{"zero", 0, 0},
		{"whole unit", 1, 10000},
		{"typical", 420.67, 4206700},
		{"one pip", 0.0001, 1},
		{"rounds half up", 0.00005, 1},
		{"rounds down", 0.00004, 0},
		{"negative", -1.2345, -12345},
		{"negative rounds", -0.00005, -1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := types.PriceFromFloat(tc.in)
			if got != tc.want {
				t.Errorf("PriceFromFloat(%v) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestPrice_Float(t *testing.T) {
	tests := []struct {
		name string
		in   types.Price
		want float64
	}{
		{"zero", 0, 0},
		{"whole unit", 10000, 1},
		{"typical", 4206700, 420.67},
		{"one pip", 1, 0.0001},
		{"negative", -12345, -1.2345},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in.Float()
			if got != tc.want {
				t.Errorf("Price(%d).Float() = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestPrice_RoundTrip(t *testing.T) {
	values := []float64{0, 1, 420.67, 0.0001, -1.2345, 99999.9999}

	for _, v := range values {
		got := types.PriceFromFloat(v).Float()
		if got != v {
			t.Errorf("round-trip %v: got %v", v, got)
		}
	}
}

func TestParsePrice(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    types.Price
		wantErr bool
	}{
		{"integer only", "130", 1300000, false},
		{"short fraction", "130.5", 1305000, false},
		{"exact-4 fraction", "130.1234", 1301234, false},
		{"too many decimals truncates", "130.12345", 1301234, false},
		{"negative", "-130.5", -1305000, false},
		{"empty string", "", 0, true},
		{"dot alone", ".", 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := types.ParsePrice(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Errorf("ParsePrice(%q) = %d, nil; want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Errorf("ParsePrice(%q) unexpected error: %v", tc.in, err)
				return
			}
			if got != tc.want {
				t.Errorf("ParsePrice(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestPrice_String(t *testing.T) {
	tests := []struct {
		name string
		in   types.Price
		want string
	}{
		{"zero", 0, "0.0000"},
		{"whole unit", 10000, "1.0000"},
		{"typical", 4206700, "420.6700"},
		{"one pip", 1, "0.0001"},
		{"negative", -12345, "-1.2345"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in.String()
			if got != tc.want {
				t.Errorf("Price(%d).String() = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
