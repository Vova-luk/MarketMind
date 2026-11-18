package priceformat

import (
	"math"
	"strconv"
)

// TwoDecimals formats a price for display. Unexpected values are passed through.
func TwoDecimals(raw string) string {
	price, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return raw
	}
	scaled := price * 100
	if math.IsNaN(scaled) || math.IsInf(scaled, 0) {
		return raw
	}
	return strconv.FormatFloat(math.Round(scaled)/100, 'f', 2, 64)
}

func OptionalTwoDecimals(raw *string) *string {
	if raw == nil {
		return nil
	}
	formatted := TwoDecimals(*raw)
	return &formatted
}
