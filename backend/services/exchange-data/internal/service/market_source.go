package service

import "context"

type MarketToCheck struct {
	MarketID   string
	ExternalID string
}

type ExchangeMarkets struct {
	ExchangeName string
	Markets      []MarketToCheck
}

type MarketSource interface {
	GetMarketsToCheck(ctx context.Context) ([]ExchangeMarkets, error)
}
