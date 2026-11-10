package okx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"marketmind/exchange-data/internal/domain"
	"marketmind/exchange-data/internal/utils"
)

func (c *OKXClient) ListInstruments(ctx context.Context) ([]domain.Instrument, error) {
	var response struct {
		Code    string `json:"code"`
		Message string `json:"msg"`
		Data    []struct {
			ID       string `json:"instId"`
			BaseCcy  string `json:"baseCcy"`
			QuoteCcy string `json:"quoteCcy"`
			State    string `json:"state"`
		} `json:"data"`
	}
	err := c.get(ctx, "/api/v5/public/instruments?instType=SPOT", func(body io.Reader) error {
		return json.NewDecoder(body).Decode(&response)
	})
	if err != nil {
		return nil, err
	}
	if response.Code != "0" {
		return nil, fmt.Errorf("OKX returned code %q: %s", response.Code, response.Message)
	}
	if response.Data == nil {
		return nil, fmt.Errorf("OKX response is missing data")
	}
	instruments := make([]domain.Instrument, 0, len(response.Data))
	for _, instrument := range response.Data {
		if instrument.State == "live" {
			instruments = append(instruments, domain.Instrument{
				ExternalID: instrument.ID, BaseAsset: instrument.BaseCcy, QuoteAsset: instrument.QuoteCcy,
			})
		}
	}
	return instruments, nil
}

func (c *OKXClient) ListPrices(ctx context.Context) (map[string]string, error) {
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

func (c *OKXClient) get(ctx context.Context, path string, decode func(io.Reader) error) error {
	cfg := utils.RetryConfig{MaxAttempts: c.maxAttempts, Delay: c.retryDelay}
	return utils.Get(ctx, c.client, strings.TrimRight(c.baseURL, "/")+path, cfg, c.logger, decode)
}
