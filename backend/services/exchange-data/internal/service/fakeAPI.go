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

type StubMarketSource struct{}

func (s *StubMarketSource) GetMarketsToCheck(ctx context.Context) ([]ExchangeMarkets, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	return []ExchangeMarkets{
		{
			ExchangeName: "binance",
			Markets: []MarketToCheck{
				{MarketID: "42", ExternalID: "BTCUSDT"},
			},
		},
		{
			ExchangeName: "bybit",
			Markets:      []MarketToCheck{{MarketID: "43", ExternalID: "BTCUSDT"}},
		},
		{
			ExchangeName: "okx",
			Markets: []MarketToCheck{
				{MarketID: "44", ExternalID: "BTC-USDT"},
			},
		},
	}, nil
}
