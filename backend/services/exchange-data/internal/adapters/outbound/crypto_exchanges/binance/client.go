package binance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"marketmind/exchange-data/internal/adapters/outbound/crypto_exchanges/binance/dto"
	"marketmind/exchange-data/internal/config"
	"marketmind/exchange-data/internal/domain"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const exchangeName = "Binance"

type BinanceClient struct {
	baseURL     string
	retryDelay  time.Duration
	maxAttempts int

	client *http.Client
	logger *slog.Logger
}

func NewBinanceClient(cfg config.ExchangeConfig, logger *slog.Logger) *BinanceClient {
	if logger == nil {
		logger = slog.Default()
	}

	return &BinanceClient{
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
		baseURL:     cfg.BaseURL,
		retryDelay:  cfg.RetryDelay,
		maxAttempts: cfg.MaxAttempts,

		logger: logger,
	}
}

func (c *BinanceClient) GetActualCryptocurrencyPriceFromBinance(ctx context.Context, symbols []string) (*domain.Market, error) {
	if len(symbols) == 0 {
		return nil, fmt.Errorf("symbols must not be empty")
	}

	validSymbols := symbols[:0]

	for _, symbol := range symbols {
		symbol = strings.TrimSpace(symbol)
		if symbol == "" {
			continue
		}

		validSymbols = append(validSymbols, symbol)
	}

	if len(validSymbols) == 0 {
		return nil, fmt.Errorf("no non-empty symbols provided")
	}

	params := url.Values{}

	if len(validSymbols) == 1 {
		params.Set("symbol", validSymbols[0])
	} else {
		data, err := json.Marshal(validSymbols)
		if err != nil {
			return nil, fmt.Errorf("encode valid symbols: %w", err)
		}

		params.Set("symbols", string(data))
	}

	URL := c.baseURL + "/api/v3/ticker/price?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.doWithRetry(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	var responsePrices []dto.ExchangePrice
	decoder := json.NewDecoder(resp.Body)

	if len(validSymbols) == 1 {
		var price dto.ExchangePrice
		if err := decoder.Decode(&price); err != nil {
			return nil, fmt.Errorf("decode Binance price: %w", err)
		}

		responsePrices = []dto.ExchangePrice{price}
	} else {
		if err := decoder.Decode(&responsePrices); err != nil {
			return nil, fmt.Errorf("decode Binance prices: %w", err)
		}
	}

	cryptocurrencies := make([]domain.Cryptocurrency, len(responsePrices))

	for ind, cryptocurrency := range responsePrices {
		var quoteError string

		switch {
		case cryptocurrency.Symbol == "":
			quoteError = "Binance returned an empty symbol"
		default:
			if err := validatePrice(cryptocurrency.Price); err != nil {
				quoteError = err.Error()
			}
		}

		cryptocurrencies[ind] = domain.Cryptocurrency{
			Symbol: cryptocurrency.Symbol,
			Price:  cryptocurrency.Price,
			Error:  quoteError,
		}
	}

	market := domain.Market{
		Exchange:       exchangeName,
		Cryptocurrency: cryptocurrencies,
		ReceivedAt:     time.Now().UTC(),
	}

	if len(market.Cryptocurrency) == 0 {
		return nil, fmt.Errorf("service didn't find any prices")
	}

	return &market, nil
}

func (c *BinanceClient) doWithRetry(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	if c.maxAttempts < 1 {
		return nil, fmt.Errorf("Binance max_attempts must be at least 1")
	}
	var lastErr error

	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		resp, err := c.client.Do(req)

		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			var netErr net.Error
			if !errors.As(err, &netErr) || !netErr.Timeout() {
				return nil, fmt.Errorf("send Binance request: %w", err)
			}

			lastErr = err
		} else {
			switch resp.StatusCode {
			case http.StatusOK:
				return resp, nil

			case http.StatusBadGateway,
				http.StatusServiceUnavailable,
				http.StatusGatewayTimeout:
				lastErr = fmt.Errorf(
					"Binance returned HTTP %d", resp.StatusCode,
				)
				resp.Body.Close()

			default:
				resp.Body.Close()
				return nil, fmt.Errorf(
					"Binance returned HTTP %d", resp.StatusCode,
				)
			}
		}

		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if attempt == c.maxAttempts {
			break
		}

		c.logger.WarnContext(ctx, "Binance request failed; retrying",
			slog.Int("attempt", attempt),
			slog.Duration("retry_delay", c.retryDelay),
			slog.Any("error", lastErr),
		)

		timer := time.NewTimer(c.retryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	return nil, fmt.Errorf(
		"Binance request failed after %d attempts: %w",
		c.maxAttempts, lastErr,
	)
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
