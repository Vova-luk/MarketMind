package markets

import (
	"context"
	"errors"
	"fmt"

	"marketmind/api/internal/domain/exchanges"
	"marketmind/api/internal/domain/markets"
)

var ErrCatalogUnavailable = errors.New("market catalog unavailable")

type ExchangeReader interface {
	GetActive(ctx context.Context, id int32) (exchanges.Exchange, error)
}

// Catalog returns one page of instruments matching a query.
type Catalog interface {
	Search(ctx context.Context, exchangeName, query string, page, limit int32) (Page, error)
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
	page, err := s.catalog.Search(ctx, exchange.Name, query.Text, query.Page, query.Limit)
	if err != nil {
		return Page{}, fmt.Errorf("%w: %w", ErrCatalogUnavailable, err)
	}

	return page, nil
}
