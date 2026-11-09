package markets

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"marketmind/api/internal/domain/exchanges"
	"marketmind/api/internal/domain/markets"
)

var ErrCatalogUnavailable = errors.New("market catalog unavailable")

type ExchangeReader interface {
	GetActive(ctx context.Context, id int32) (exchanges.Exchange, error)
}

// Catalog returns the instruments matching a query in one call.
type Catalog interface {
	Search(ctx context.Context, exchange exchanges.Exchange, query string) ([]markets.Instrument, error)
}

type Service struct {
	exchanges ExchangeReader
	catalog   Catalog
}

func NewService(exchanges ExchangeReader, catalog Catalog) *Service {
	return &Service{exchanges: exchanges, catalog: catalog}
}

type SearchQuery struct {
	ExchangeID int32
	Text       string
	Page       int32
	Limit      int32
}

type Page struct {
	Items    []markets.Instrument
	NextPage *int32
}

func (s *Service) Search(ctx context.Context, query SearchQuery) (Page, error) {
	exchange, err := s.exchanges.GetActive(ctx, query.ExchangeID)
	if err != nil {
		return Page{}, err
	}
	matches, err := s.catalog.Search(ctx, exchange, query.Text)
	if err != nil {
		return Page{}, fmt.Errorf("%w: %w", ErrCatalogUnavailable, err)
	}

	matches = slices.Clone(matches)
	slices.SortStableFunc(matches, func(a, b markets.Instrument) int {
		return strings.Compare(a.ExternalID, b.ExternalID)
	})
	matches = slices.CompactFunc(matches, func(a, b markets.Instrument) bool {
		return a.ExternalID == b.ExternalID
	})

	start := int64(query.Page-1) * int64(query.Limit)
	if start >= int64(len(matches)) {
		return Page{}, nil
	}
	end := min(start+int64(query.Limit), int64(len(matches)))
	result := Page{Items: matches[start:end]}
	if end < int64(len(matches)) {
		nextPage := query.Page + 1
		result.NextPage = &nextPage
	}
	return result, nil
}
