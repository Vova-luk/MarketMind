package exchangedata

import (
	"context"
	"time"

	"marketmind/api/internal/domain/markets"
	marketservice "marketmind/api/internal/service/markets"
	"marketmind/gen/go/exchangedata"
)

type Catalog struct {
	client  exchangedata.MarketSearchServiceClient
	timeout time.Duration
}

func NewCatalog(client exchangedata.MarketSearchServiceClient, timeout time.Duration) *Catalog {
	return &Catalog{client: client, timeout: timeout}
}

func (c *Catalog) Search(ctx context.Context, exchangeName, query string, page, limit int32) (marketservice.Page, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	response, err := c.client.SearchMarkets(ctx, &exchangedata.SearchMarketsRequest{
		ExchangeName: exchangeName, Query: query, Page: &page, Limit: &limit,
	})
	if err != nil {
		return marketservice.Page{}, err
	}
	result := marketservice.Page{Items: make([]markets.Instrument, 0, len(response.Items)), NextPage: response.NextPage}
	for _, item := range response.Items {
		result.Items = append(result.Items, markets.Instrument{
			ExternalID: item.GetExternalId(), BaseAsset: item.GetBaseAsset(), QuoteAsset: item.GetQuoteAsset(), Price: item.Price,
		})
	}
	return result, nil
}
