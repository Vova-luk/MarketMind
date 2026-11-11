package markets

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"marketmind/api/internal/domain/exchanges"
	service "marketmind/api/internal/service/markets"
	"marketmind/api/internal/utils/httpresponse"
	"marketmind/utils/priceformat"
)

type MarketSearcher interface {
	Search(ctx context.Context, query service.SearchQuery) (service.Page, error)
}

type Handler struct {
	service MarketSearcher
	logger  *slog.Logger
}

func NewHandler(service MarketSearcher, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

type marketOption struct {
	ExchangeID  int32   `json:"exchange_id"`
	ExternalID  string  `json:"external_id"`
	BaseAsset   string  `json:"base_asset"`
	QuoteAsset  string  `json:"quote_asset"`
	DisplayName string  `json:"display_name"`
	Price       *string `json:"price"`
	PriceExact  *string `json:"price_exact"`
}

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	query, ok := readSearchQuery(r.URL.RawQuery)
	if !ok {
		httpresponse.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid market search parameters")
		return
	}
	ctx := r.Context()
	page, err := h.service.Search(ctx, query)
	switch {
	case errors.Is(err, exchanges.ErrNotFound):
		httpresponse.WriteError(w, http.StatusNotFound, "EXCHANGE_NOT_FOUND", "Exchange not found")
		return
	case errors.Is(err, service.ErrCatalogUnavailable):
		h.logger.ErrorContext(ctx, "search exchange catalog", "error", err)
		httpresponse.WriteError(w, http.StatusServiceUnavailable, "MARKET_CATALOG_UNAVAILABLE", "Market catalog is unavailable")
		return
	case err != nil:
		h.logger.ErrorContext(ctx, "search markets", "error", err)
		httpresponse.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Could not search markets")
		return
	}

	items := make([]marketOption, 0, len(page.Items))
	for _, instrument := range page.Items {
		items = append(items, marketOption{
			ExchangeID: query.ExchangeID, ExternalID: instrument.ExternalID,
			BaseAsset: instrument.BaseAsset, QuoteAsset: instrument.QuoteAsset,
			DisplayName: instrument.BaseAsset + "/" + instrument.QuoteAsset,
			Price:       priceformat.OptionalTwoDecimals(instrument.Price), PriceExact: instrument.Price,
		})
	}
	httpresponse.WriteJSON(w, http.StatusOK, struct {
		Items    []marketOption `json:"items"`
		NextPage *int32         `json:"next_page"`
	}{Items: items, NextPage: page.NextPage})
}

func readSearchQuery(raw string) (service.SearchQuery, bool) {
	params, err := url.ParseQuery(raw)
	if err != nil {
		return service.SearchQuery{}, false
	}
	for key, values := range params {
		if len(values) != 1 {
			return service.SearchQuery{}, false
		}
		switch key {
		case "exchange_id", "q", "page", "limit":
		default:
			return service.SearchQuery{}, false
		}
	}
	q := params.Get("q")
	exchangeID, validExchange := queryInt(params, "exchange_id", 0, math.MaxInt32)
	page, validPage := queryInt(params, "page", 1, math.MaxInt32)
	limit, validLimit := queryInt(params, "limit", 20, 100)
	if !validExchange || !validPage || !validLimit || !utf8.ValidString(q) || utf8.RuneCountInString(q) > 120 {
		return service.SearchQuery{}, false
	}
	return service.SearchQuery{ExchangeID: exchangeID, Text: strings.TrimSpace(q), Page: page, Limit: limit}, true
}

func queryInt(params url.Values, name string, defaultValue, maxValue int32) (int32, bool) {
	if !params.Has(name) {
		return defaultValue, defaultValue > 0
	}
	n, err := strconv.ParseInt(params.Get(name), 10, 32)
	return int32(n), err == nil && n > 0 && n <= int64(maxValue)
}
