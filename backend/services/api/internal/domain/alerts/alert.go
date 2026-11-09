package alerts

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"regexp"
)

const (
	Price   = "price"
	Percent = "percent"
)

var (
	ErrNotFound     = errors.New("alert not found")
	ErrInvalid      = errors.New("invalid alert condition")
	ErrExchangeOff  = errors.New("exchange is inactive")
	ErrQuoteMissing = errors.New("positive market price is unavailable")
	ErrActiveLimit  = errors.New("active alert limit reached")
	decimalPattern  = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?$`)
)

type Condition struct {
	Type  string
	Value string
}

func (c Condition) Validate() error {
	value, ok := ParseDecimal(c.Value)
	if !ok {
		return fmt.Errorf("%w: value must be a decimal string", ErrInvalid)
	}
	switch c.Type {
	case Price:
		if value.Sign() > 0 && value.Cmp(big.NewRat(500000, 1)) <= 0 {
			return nil
		}
	case Percent:
		if value.Sign() != 0 && value.Cmp(big.NewRat(-100, 1)) > 0 && value.Cmp(big.NewRat(500000, 1)) <= 0 {
			return nil
		}
	default:
		return fmt.Errorf("%w: type must be price or percent", ErrInvalid)
	}
	return fmt.Errorf("%w: value is outside the allowed range", ErrInvalid)
}

type Alert struct {
	ID int64
	Condition
	ReferencePrice string
	IsActive       bool
}

type Subscription struct {
	Price          string
	QuoteAsset     string
	ExchangeActive bool
}

type Repository interface {
	// BeginSubscription locks the owned subscription and serializes alert writes for its user.
	BeginSubscription(ctx context.Context, telegramUserID, subscriptionID int64) (Subscription, Transaction, error)
}

type Transaction interface {
	Get(ctx context.Context, alertID int64) (Alert, error)
	Insert(ctx context.Context, alert Alert) (int64, error)
	Update(ctx context.Context, alert Alert) error
	Delete(ctx context.Context, alertID int64) error
	CountActive(ctx context.Context) (int, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

func ParseDecimal(value string) (*big.Rat, bool) {
	if !decimalPattern.MatchString(value) {
		return nil, false
	}
	return new(big.Rat).SetString(value)
}

func (a Alert) TargetPrice() (string, error) {
	value, ok := ParseDecimal(a.Value)
	if !ok {
		return "", fmt.Errorf("invalid stored alert value")
	}
	switch a.Type {
	case Price:
		return a.Value, nil
	case Percent:
		reference, ok := ParseDecimal(a.ReferencePrice)
		if !ok {
			return "", fmt.Errorf("invalid stored alert reference price")
		}
		factor := new(big.Rat).Quo(value, big.NewRat(100, 1))
		factor.Add(factor, big.NewRat(1, 1))
		target := new(big.Rat).Mul(reference, factor)
		digits, _ := target.FloatPrec()
		return target.FloatString(digits), nil
	default:
		return "", fmt.Errorf("unknown stored alert type %q", a.Type)
	}
}
