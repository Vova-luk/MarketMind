package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"marketmind/exchange-data/internal/domain"
)

const (
	instrumentsTTL = time.Minute
	pricesTTL      = 5 * time.Second
)

var (
	ErrUnknownExchange = errors.New("unknown exchange")
	decimalPrice       = regexp.MustCompile(`^(0|[1-9][0-9]*)(\.[0-9]+)?$`)
)

type SpotProvider interface {
	ListInstruments(context.Context) ([]domain.Instrument, error)
	ListPrices(context.Context) (map[string]string, error)
}

type SearchService struct {
	catalogs map[string]*spotCatalog
}

type SearchResult struct {
	domain.Instrument
	Price *string
}

type SearchPage struct {
	Items    []SearchResult
	NextPage *int32
}

func NewSearchService(exchanges map[string]SpotProvider) *SearchService {
	catalogs := make(map[string]*spotCatalog, len(exchanges))
	for name, provider := range exchanges {
		catalogs[name] = &spotCatalog{provider: provider, refresh: make(chan struct{}, 1)}
	}
	return &SearchService{catalogs: catalogs}
}

func (s *SearchService) Search(ctx context.Context, exchangeName, query string, page, limit int32) (SearchPage, error) {
	name := strings.ToLower(strings.TrimSpace(exchangeName))
	catalog, ok := s.catalogs[name]
	if !ok {
		return SearchPage{}, ErrUnknownExchange
	}
	instruments, prices, err := catalog.snapshot(ctx)
	if err != nil {
		return SearchPage{}, fmt.Errorf("search %s: %w", name, err)
	}

	query = strings.ToUpper(strings.TrimSpace(query))
	offset := int64(page-1) * int64(limit)
	if offset >= int64(len(instruments)) {
		return SearchPage{}, nil
	}
	result := SearchPage{Items: make([]SearchResult, 0, limit)}
	for _, instrument := range instruments {
		if !strings.Contains(strings.ToUpper(instrument.ExternalID), query) &&
			!strings.Contains(strings.ToUpper(instrument.BaseAsset+"/"+instrument.QuoteAsset), query) {
			continue
		}
		if offset > 0 {
			offset--
			continue
		}
		if len(result.Items) == int(limit) {
			next := page + 1
			result.NextPage = &next
			break
		}
		item := SearchResult{Instrument: instrument}
		price := prices[instrument.ExternalID]
		if len(price) <= 256 && decimalPrice.MatchString(price) && strings.ContainsAny(price, "123456789") {
			item.Price = &price
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}

type spotCatalog struct {
	provider SpotProvider
	// The channel lets callers stop waiting for another request's refresh.
	refresh          chan struct{}
	instruments      []domain.Instrument
	instrumentsUntil time.Time
	prices           map[string]string
	pricesUntil      time.Time
}

func (c *spotCatalog) snapshot(ctx context.Context) ([]domain.Instrument, map[string]string, error) {
	select {
	case c.refresh <- struct{}{}:
		defer func() { <-c.refresh }()
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}

	now := time.Now()
	group, ctx := errgroup.WithContext(ctx)
	if !now.Before(c.instrumentsUntil) {
		group.Go(func() error {
			instruments, err := c.provider.ListInstruments(ctx)
			if err != nil {
				return fmt.Errorf("load spot instruments: %w", err)
			}
			instruments = slices.DeleteFunc(instruments, func(i domain.Instrument) bool {
				return i.ExternalID == "" || i.BaseAsset == "" || i.QuoteAsset == ""
			})
			slices.SortFunc(instruments, func(a, b domain.Instrument) int {
				return strings.Compare(a.ExternalID, b.ExternalID)
			})
			c.instruments = slices.CompactFunc(instruments, func(a, b domain.Instrument) bool {
				return a.ExternalID == b.ExternalID
			})
			c.instrumentsUntil = time.Now().Add(instrumentsTTL)
			return nil
		})
	}
	if !now.Before(c.pricesUntil) {
		group.Go(func() error {
			prices, err := c.provider.ListPrices(ctx)
			if err != nil {
				return fmt.Errorf("load spot prices: %w", err)
			}
			c.prices = prices
			c.pricesUntil = time.Now().Add(pricesTTL)
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return nil, nil, err
	}
	return c.instruments, c.prices, nil
}
