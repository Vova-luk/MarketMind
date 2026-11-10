package service

import (
	"fmt"
	"marketmind/tracking/internal/domain"
	"math/big"
	"regexp"
)

var decimalPattern = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)

func decimal(value string) (*big.Rat, error) {
	if !decimalPattern.MatchString(value) {
		return nil, fmt.Errorf("invalid decimal %q", value)
	}
	number, ok := new(big.Rat).SetString(value)
	if !ok {
		return nil, fmt.Errorf("invalid decimal %q", value)
	}
	return number, nil
}

func positiveDecimal(value string) (*big.Rat, error) {
	number, err := decimal(value)
	if err != nil {
		return nil, err
	}
	if number.Sign() <= 0 {
		return nil, fmt.Errorf("price must be positive")
	}
	return number, nil
}

func threshold(alert domain.Alert) (*big.Rat, error) {
	value, err := decimal(alert.Value)
	if err != nil {
		return nil, err
	}
	switch alert.Type {
	case "price":
		return positiveDecimal(alert.Value)
	case "percent":
		if value.Sign() == 0 || value.Cmp(big.NewRat(-100, 1)) <= 0 {
			return nil, fmt.Errorf("invalid percent value")
		}
		reference, err := positiveDecimal(alert.ReferencePrice)
		if err != nil {
			return nil, err
		}
		factor := new(big.Rat).Quo(value, big.NewRat(100, 1))
		factor.Add(factor, big.NewRat(1, 1))
		return new(big.Rat).Mul(reference, factor), nil
	default:
		return nil, fmt.Errorf("unknown alert type %q", alert.Type)
	}
}

func matches(price, target *big.Rat, direction string) (bool, error) {
	switch direction {
	case "gte":
		return price.Cmp(target) >= 0, nil
	case "lte":
		return price.Cmp(target) <= 0, nil
	default:
		return false, fmt.Errorf("unknown alert direction %q", direction)
	}
}
