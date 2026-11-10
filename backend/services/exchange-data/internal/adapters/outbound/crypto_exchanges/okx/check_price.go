// REST market data: https://www.okx.com/docs-v5/en/#order-book-trading-market-data-get-tickers
package okx

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"marketmind/exchange-data/internal/adapters/outbound/crypto_exchanges/okx/dto"
	"marketmind/exchange-data/internal/config"
	"marketmind/exchange-data/internal/domain"
)

const exchangeName = "OKX"

type OKXClient struct {
	baseURL     string
	retryDelay  time.Duration
	maxAttempts int
	logger      *slog.Logger
	client      *http.Client
}

func NewOKXClient(cfg config.ExchangeConfig, logger *slog.Logger) *OKXClient {
	if logger == nil {
		logger = slog.Default()
	}

	return &OKXClient{
		baseURL:     cfg.BaseURL,
		retryDelay:  cfg.RetryDelay,
		maxAttempts: cfg.MaxAttempts,
		logger:      logger,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

func (c *OKXClient) GetPrices(ctx context.Context, symbols []string) (*domain.Market, error) {
	started := time.Now()
	wanted := make(map[string]int, len(symbols))

	validSymbols := symbols[:0]
	for _, symbol := range symbols {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			continue
		}
		if _, exists := wanted[symbol]; exists {
			continue
		}
		wanted[symbol] = len(validSymbols)
		validSymbols = append(validSymbols, symbol)
	}
	if len(validSymbols) == 0 {
		return nil, fmt.Errorf("no non-empty symbols provided")
	}

	prices, err := c.fetchTickers(ctx)
	if err != nil {
		return nil, err
	}
	market := mapPrices(prices, validSymbols, wanted)
	failed := 0
	for _, quote := range market.Cryptocurrency {
		if quote.Error != "" {
			failed++
		}
	}

	c.logger.DebugContext(ctx, "OKX quotes fetched",
		slog.String("exchange", exchangeName),
		slog.Int("requested", len(validSymbols)),
		slog.Int("received", len(prices)),
		slog.Int("valid", len(validSymbols)-failed),
		slog.Int("failed", failed),
		slog.Duration("duration", time.Since(started)),
	)

	return market, nil
}

func (c *OKXClient) fetchTickers(ctx context.Context) ([]dto.ExchangePrice, error) {
	var prices []dto.ExchangePrice
	err := c.get(ctx, "/api/v5/market/tickers?instType=SPOT", func(body io.Reader) error {
		var err error
		prices, err = decodePrices(body)
		return err
	})
	return prices, err
}

func decodePrices(body io.Reader) ([]dto.ExchangePrice, error) {
	var response dto.TickersResponse
	if err := json.NewDecoder(body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode OKX prices: %w", err)
	}
	if response.Code != "0" {
		return nil, fmt.Errorf("OKX returned code %q: %s", response.Code, response.Message)
	}
	if response.Data == nil {
		return nil, fmt.Errorf("OKX response is missing data")
	}
	return response.Data, nil
}

func mapPrices(prices []dto.ExchangePrice, symbols []string, wanted map[string]int) *domain.Market {
	quotes := make([]domain.Cryptocurrency, len(symbols))
	for i, symbol := range symbols {
		quotes[i] = domain.Cryptocurrency{Symbol: symbol, Error: "OKX returned no quote for requested symbol"}
	}
	for _, price := range prices {
		index, ok := wanted[price.Symbol]
		if !ok {
			continue
		}
		quote := domain.Cryptocurrency{Symbol: price.Symbol, Price: price.Price}
		if err := validatePrice(price.Price); err != nil {
			quote.Error = err.Error()
		}
		quotes[index] = quote
	}
	return &domain.Market{Exchange: exchangeName, Cryptocurrency: quotes, ReceivedAt: time.Now().UTC()}
}

func validatePrice(value string) error {
	price, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("invalid price %q: %w", value, err)
	}

	if math.IsNaN(price) || math.IsInf(price, 0) || price <= 0 {
		return fmt.Errorf("price must be a finite positive number: %q", value)
	}

	return nil
}
