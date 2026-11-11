package marketcatalog

import (
	"context"
	"time"

	"marketmind/exchange-data/internal/service"
	"marketmind/gen/go/api"
)

type Client struct {
	client  api.MarketCatalogServiceClient
	timeout time.Duration
}

func NewClient(client api.MarketCatalogServiceClient, timeout time.Duration) *Client {
	return &Client{client: client, timeout: timeout}
}

func (c *Client) GetMarketsToCheck(ctx context.Context) ([]service.ExchangeMarkets, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.client.GetMarketsToCheck(ctx, &api.GetMarketsToCheckRequest{})
	if err != nil {
		return nil, err
	}

	exchanges := make([]service.ExchangeMarkets, 0, len(response.GetItems()))
	for _, item := range response.GetItems() {
		exchange := service.ExchangeMarkets{
			ExchangeName: item.GetExchangeName(),
			Markets:      make([]service.MarketToCheck, 0, len(item.GetMarkets())),
		}
		for _, market := range item.GetMarkets() {
			exchange.Markets = append(exchange.Markets, service.MarketToCheck{
				MarketID: market.GetMarketId(), ExternalID: market.GetExternalId(),
			})
		}
		exchanges = append(exchanges, exchange)
	}
	return exchanges, nil
}
