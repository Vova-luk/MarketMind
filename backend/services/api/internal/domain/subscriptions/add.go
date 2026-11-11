package subscriptions

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	ErrUserNotRegistered = errors.New("user is not registered")
	ErrExchangeNotFound  = errors.New("exchange not found")
	ErrExchangeInactive  = errors.New("exchange is inactive")
	ErrLimitReached      = errors.New("subscription limit reached")
	ErrMarketConflict    = errors.New("pair already has a different external ID")
	ErrInvalidMarketData = errors.New("invalid market data")
	decimalPrice         = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]+)?$`)
)

type MarketKey struct {
	ExchangeID int32
	ExternalID string
}

func (k MarketKey) Valid() bool {
	return k.ExchangeID > 0 && validCode(k.ExternalID, 255)
}

type MarketSeed struct {
	BaseAsset  string
	QuoteAsset string
	Price      string
}

func (s MarketSeed) Validate() error {
	if !validCode(s.BaseAsset, 32) || !validCode(s.QuoteAsset, 32) ||
		strings.Contains(s.BaseAsset, "/") || strings.Contains(s.QuoteAsset, "/") {
		return fmt.Errorf("%w: base_asset and quote_asset are required", ErrInvalidMarketData)
	}
	if len(s.Price) > 256 || !decimalPrice.MatchString(s.Price) || !strings.ContainsAny(s.Price, "123456789") {
		return fmt.Errorf("%w: price must be a positive decimal string", ErrInvalidMarketData)
	}
	return nil
}

func validCode(code string, maxLength int) bool {
	return code != "" && utf8.ValidString(code) && utf8.RuneCountInString(code) <= maxLength &&
		strings.IndexFunc(code, func(r rune) bool {
			return unicode.IsSpace(r) || unicode.IsControl(r) || r == '\uFEFF'
		}) == -1
}

type Exchange struct {
	Name   string
	Active bool
}

type Market struct {
	MarketID int64
	Subscription
}

type AddRepository interface {
	BeginAdd(ctx context.Context, telegramUserID int64) (AddTransaction, error)
}

type AddTransaction interface {
	GetExchange(ctx context.Context, exchangeID int32) (Exchange, error)
	FindMarket(ctx context.Context, key MarketKey, exchange Exchange) (Market, error)
	CountSubscriptions(ctx context.Context) (int, error)
	InsertMarket(ctx context.Context, key MarketKey, exchange Exchange, seed MarketSeed) (Market, error)
	InsertSubscription(ctx context.Context, marketID int64) (int64, error)
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}
