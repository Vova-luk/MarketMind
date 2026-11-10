package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"marketmind/exchange-data/internal/domain"
	"marketmind/exchange-data/internal/utils"
)

func (c *BinanceClient) ListInstruments(ctx context.Context) ([]domain.Instrument, error) {
	var response struct {
		Symbols []struct {
			Symbol     string `json:"symbol"`
			BaseAsset  string `json:"baseAsset"`
			QuoteAsset string `json:"quoteAsset"`
			Status     string `json:"status"`
			Spot       bool   `json:"isSpotTradingAllowed"`
		} `json:"symbols"`
	}
	err := c.get(ctx, "/api/v3/exchangeInfo?permissions=SPOT&symbolStatus=TRADING&showPermissionSets=false", func(body io.Reader) error {
		return json.NewDecoder(body).Decode(&response)
	})
	if err != nil {
		return nil, err
	}
	if response.Symbols == nil {
		return nil, fmt.Errorf("Binance response is missing symbols")
	}
	instruments := make([]domain.Instrument, 0, len(response.Symbols))
	for _, symbol := range response.Symbols {
		if symbol.Status == "TRADING" && symbol.Spot {
			instruments = append(instruments, domain.Instrument{
				ExternalID: symbol.Symbol, BaseAsset: symbol.BaseAsset, QuoteAsset: symbol.QuoteAsset,
			})
		}
	}
	return instruments, nil
}

func (c *BinanceClient) ListPrices(ctx context.Context) (map[string]string, error) {
	tickers, err := c.fetchTickers(ctx, nil)
	if err != nil {
		return nil, err
	}
	if tickers == nil {
		return nil, fmt.Errorf("Binance response is missing prices")
	}
	prices := make(map[string]string, len(tickers))
	for _, ticker := range tickers {
		prices[ticker.Symbol] = ticker.Price
	}
	return prices, nil
}

func (c *BinanceClient) get(ctx context.Context, path string, decode func(io.Reader) error) error {
	cfg := utils.RetryConfig{MaxAttempts: c.maxAttempts, Delay: c.retryDelay}
	return utils.Get(ctx, c.client, strings.TrimRight(c.baseURL, "/")+path, cfg, c.logger, decode)
}
