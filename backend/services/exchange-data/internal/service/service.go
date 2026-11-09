package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"marketmind/exchange-data/internal/adapters/outbound/kafka_producer/events"
	"marketmind/exchange-data/internal/domain"
)

type exchangeResult struct {
	prices *domain.Market
	err    error
}

type PriceProvider interface {
	GetPrices(ctx context.Context, symbols []string) (*domain.Market, error)
}

type Publisher interface {
	SendPriceChecked(ctx context.Context, event events.PricesCheckedEvent) error
}

type Service struct {
	exchanges map[string]PriceProvider
	publisher Publisher
	fakeAPI   MarketSource
	logger    *slog.Logger
}

func NewService(exchanges map[string]PriceProvider, publisher Publisher, marketSource MarketSource, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		exchanges: exchanges,
		publisher: publisher,
		fakeAPI:   marketSource,
		logger:    logger,
	}
}

func (s *Service) CheckPrices(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	items, err := s.fakeAPI.GetMarketsToCheck(ctx)
	if err != nil {
		return fmt.Errorf("get markets to check: %w", err)
	}

	results := make([]exchangeResult, len(items))
	slots := make(chan struct{}, 3)

	var wg sync.WaitGroup

	for i, item := range items {
		if len(item.Markets) == 0 {
			continue
		}
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		}

		wg.Add(1)
		go func(index int, item ExchangeMarkets) {
			defer wg.Done()
			defer func() { <-slots }()
			results[index] = s.checkExchange(ctx, item)
		}(i, item)
	}
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return err
	}

	var checked []events.CheckedPrice
	var failures []error

	for i, result := range results {
		if len(items[i].Markets) == 0 {
			continue
		}
		exchange := strings.ToLower(strings.TrimSpace(items[i].ExchangeName))
		if result.err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", exchange, result.err))
			s.logger.WarnContext(ctx, "exchange price check failed",
				slog.String("exchange", exchange), slog.Any("error", result.err))
			continue
		}

		quotes, err := s.collectQuotes(ctx, exchange, items[i].Markets, result.prices)
		if err != nil {
			failures = append(failures, err)
		}

		checked = append(checked, quotes...)
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	if len(checked) == 0 {
		if len(failures) != 0 {
			return fmt.Errorf("no valid prices collected: %w", errors.Join(failures...))
		}
		return nil
	}

	event := events.PricesCheckedEvent{EventID: rand.Text(), Items: checked}
	if err := s.publisher.SendPriceChecked(ctx, event); err != nil {
		return fmt.Errorf("publish prices checked event: %w", err)
	}

	s.logger.DebugContext(ctx, "price check completed",
		slog.String("event_id", event.EventID), slog.Int("quotes", len(checked)))
	return nil
}

func (s *Service) checkExchange(ctx context.Context, item ExchangeMarkets) exchangeResult {
	if err := ctx.Err(); err != nil {
		return exchangeResult{err: err}
	}
	exchange := strings.ToLower(strings.TrimSpace(item.ExchangeName))
	client, ok := s.exchanges[exchange]

	if !ok || client == nil {
		return exchangeResult{err: fmt.Errorf("unsupported exchange: %q", exchange)}
	}

	symbols := make([]string, 0, len(item.Markets))

	for _, market := range item.Markets {
		symbol := strings.TrimSpace(market.ExternalID)
		if symbol != "" && strings.TrimSpace(market.MarketID) != "" {
			symbols = append(symbols, symbol)
		}
	}
	if len(symbols) == 0 {
		return exchangeResult{err: fmt.Errorf("no markets with both market_id and external_id")}
	}

	prices, err := client.GetPrices(ctx, symbols)
	if err != nil {
		return exchangeResult{err: err}
	}

	if prices == nil {
		return exchangeResult{err: fmt.Errorf("price provider returned a nil market")}
	}

	return exchangeResult{prices: prices}
}

func (s *Service) collectQuotes(ctx context.Context, exchange string, markets []MarketToCheck, prices *domain.Market) ([]events.CheckedPrice, error) {
	bySymbol := make(map[string]domain.Cryptocurrency, len(prices.Cryptocurrency))
	for _, quote := range prices.Cryptocurrency {
		bySymbol[quote.Symbol] = quote
	}

	checked := make([]events.CheckedPrice, 0, len(markets))
	var failures []error

	for _, market := range markets {
		symbol := strings.TrimSpace(market.ExternalID)
		quote, found := bySymbol[symbol]
		var err error
		switch {
		case strings.TrimSpace(market.MarketID) == "" || symbol == "":
			err = fmt.Errorf("market_id and external_id must not be empty")
		case !found:
			err = fmt.Errorf("price provider returned no quote")
		case quote.Error != "":
			err = errors.New(quote.Error)
		case strings.TrimSpace(quote.Price) == "":
			err = fmt.Errorf("price provider returned an empty price")
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("%s market %q (%s): %w", exchange, market.MarketID, symbol, err))
			s.logger.WarnContext(ctx, "quote skipped",
				slog.String("exchange", exchange), slog.String("market_id", market.MarketID),
				slog.String("symbol", symbol), slog.Any("error", err))
			continue
		}
		checked = append(checked, events.CheckedPrice{
			ExchangeName: exchange, MarketID: market.MarketID, ExternalID: symbol,
			Price: quote.Price, ReceivedAt: prices.ReceivedAt.UTC(),
		})
	}
	return checked, errors.Join(failures...)
}
