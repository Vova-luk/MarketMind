package binance

import (
	"context"
	"encoding/json"
	"fmt"
	"marketmind/exchange-data/internal/adapters/outbound/crypto_exchanges/binance/dto"
	"marketmind/exchange-data/internal/domain"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const exchangeName = "Binance"

type BinanceClient struct {
	client *http.Client
}

func NewBinanceClient() *BinanceClient {
	return &BinanceClient{
		&http.Client{
			Timeout: time.Second * 10,
		},
	}
}

func (c *BinanceClient) GetActualCryptocurrencyPrice(ctx context.Context, symbols []string) (*domain.Market, error) {
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

	URL := "https://data-api.binance.vision/api/v3/ticker/price?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to Binance: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Binance returned HTTP %d", resp.StatusCode)
	}

	var responsePrices []dto.BinancePrice
	decoder := json.NewDecoder(resp.Body)

	if len(validSymbols) == 1 {
		var price dto.BinancePrice
		if err := decoder.Decode(&price); err != nil {
			return nil, fmt.Errorf("decode Binance price: %w", err)
		}

		responsePrices = []dto.BinancePrice{price}
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

	market := &domain.Market{
		Exchange:       exchangeName,
		Cryptocurrency: cryptocurrencies,
		ReceivedAt:     time.Now().UTC(),
	}

	if len(market.Cryptocurrency) == 0 {
		return nil, fmt.Errorf("service didn't find any prices")
	}

	return market, nil
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
