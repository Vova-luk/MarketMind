package bybit

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"marketmind/exchange-data/internal/adapters/outbound/crypto_exchanges/bybit/dto"
	"marketmind/exchange-data/internal/config"
	"marketmind/exchange-data/internal/domain"
	"marketmind/exchange-data/internal/utils"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const exchangeName = "Bybit"

type BybitClient struct {
	baseURL     string
	retryDelay  time.Duration
	maxAttempts int
	logger      *slog.Logger
	client      *http.Client
}

func NewBybitClient(cfg config.ExchangeConfig, logger *slog.Logger) *BybitClient {
	if logger == nil {
		logger = slog.Default()
	}

	return &BybitClient{
		baseURL:     cfg.BaseURL,
		retryDelay:  cfg.RetryDelay,
		maxAttempts: cfg.MaxAttempts,
		logger:      logger,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

func (c *BybitClient) GetPrices(ctx context.Context, symbols []string) (*domain.Market, error) {
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

	requestURL := strings.TrimRight(c.baseURL, "/") + "/v5/market/tickers?category=spot"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create Bybit request: %w", err)
	}
	resp, err := utils.DoWithRetry(req, c.client, utils.RetryConfig{
		MaxAttempts: c.maxAttempts,
		Delay:       c.retryDelay,
	}, c.logger)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	prices, err := decodePrices(resp.Body)
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

	c.logger.DebugContext(ctx, "Bybit quotes fetched",
		slog.String("exchange", exchangeName),
		slog.Int("requested", len(validSymbols)),
		slog.Int("received", len(prices)),
		slog.Int("valid", len(validSymbols)-failed),
		slog.Int("failed", failed),
		slog.Duration("duration", time.Since(started)),
	)

	return market, nil
}

func decodePrices(body io.Reader) ([]dto.ExchangePrice, error) {
	var response dto.TickersResponse
	if err := json.NewDecoder(body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decode Bybit prices: %w", err)
	}
	if response.RetCode != 0 {
		return nil, fmt.Errorf("Bybit returned code %d: %s", response.RetCode, response.RetMsg)
	}
	if response.Result.List == nil {
		return nil, fmt.Errorf("Bybit response is missing result.list")
	}
	return response.Result.List, nil
}

func mapPrices(prices []dto.ExchangePrice, symbols []string, wanted map[string]int) *domain.Market {
	quotes := make([]domain.Cryptocurrency, len(symbols))
	for i, symbol := range symbols {
		quotes[i] = domain.Cryptocurrency{Symbol: symbol, Error: "Bybit returned no quote for requested symbol"}
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
