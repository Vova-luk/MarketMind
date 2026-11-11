package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"marketmind/exchange-data/internal/domain"
	"marketmind/exchange-data/internal/utils"
)

func (c *BybitClient) ListInstruments(ctx context.Context) ([]domain.Instrument, error) {
	var response struct {
		RetCode int    `json:"retCode"`
		RetMsg  string `json:"retMsg"`
		Result  struct {
			List []struct {
				Symbol    string `json:"symbol"`
				BaseCoin  string `json:"baseCoin"`
				QuoteCoin string `json:"quoteCoin"`
				Status    string `json:"status"`
			} `json:"list"`
		} `json:"result"`
	}
	err := c.get(ctx, "/v5/market/instruments-info?category=spot&status=Trading", func(body io.Reader) error {
		return json.NewDecoder(body).Decode(&response)
	})
	if err != nil {
		return nil, err
	}
	if response.RetCode != 0 {
		return nil, fmt.Errorf("Bybit returned code %d: %s", response.RetCode, response.RetMsg)
	}
	if response.Result.List == nil {
		return nil, fmt.Errorf("Bybit response is missing result.list")
	}
	instruments := make([]domain.Instrument, 0, len(response.Result.List))
	for _, instrument := range response.Result.List {
		if instrument.Status == "Trading" {
			instruments = append(instruments, domain.Instrument{
				ExternalID: instrument.Symbol, BaseAsset: instrument.BaseCoin, QuoteAsset: instrument.QuoteCoin,
			})
		}
	}
	return instruments, nil
}

func (c *BybitClient) ListPrices(ctx context.Context) (map[string]string, error) {
	tickers, err := c.fetchTickers(ctx)
	if err != nil {
		return nil, err
	}
	prices := make(map[string]string, len(tickers))
	for _, ticker := range tickers {
		prices[ticker.Symbol] = ticker.Price
	}
	return prices, nil
}

func (c *BybitClient) get(ctx context.Context, path string, decode func(io.Reader) error) error {
	cfg := utils.RetryConfig{MaxAttempts: c.maxAttempts, Delay: c.retryDelay}
	return utils.Get(ctx, c.client, strings.TrimRight(c.baseURL, "/")+path, cfg, c.logger, decode)
}
