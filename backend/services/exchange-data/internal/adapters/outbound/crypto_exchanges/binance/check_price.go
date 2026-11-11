package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"marketmind/exchange-data/internal/adapters/outbound/crypto_exchanges/binance/dto"
	"marketmind/exchange-data/internal/config"
	"marketmind/exchange-data/internal/domain"
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

func (c *BinanceClient) GetPrices(ctx context.Context, symbols []string) (*domain.Market, error) {
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

	responsePrices, err := c.fetchTickers(ctx, params)
	if err != nil {
		return nil, err
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

func (c *BinanceClient) fetchTickers(ctx context.Context, params url.Values) ([]dto.ExchangePrice, error) {
	path := "/api/v3/ticker/price"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	var prices []dto.ExchangePrice
	err := c.get(ctx, path, func(body io.Reader) error {
		decoder := json.NewDecoder(body)
		if params.Has("symbol") {
			var price dto.ExchangePrice
			if err := decoder.Decode(&price); err != nil {
				return err
			}
			prices = []dto.ExchangePrice{price}
			return nil
		}
		return decoder.Decode(&prices)
	})
	return prices, err
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
