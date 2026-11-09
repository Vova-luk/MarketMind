package application

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"marketmind/api/internal/adapters/exchangedata"
	domainexchanges "marketmind/api/internal/domain/exchanges"
	markethandlers "marketmind/api/internal/handlers/markets"
	userhandlers "marketmind/api/internal/handlers/users"
	marketservice "marketmind/api/internal/service/markets"
)

type activeExchangeStub struct{}

func (activeExchangeStub) GetActive(_ context.Context, id int32) (domainexchanges.Exchange, error) {
	if id != 7 {
		return domainexchanges.Exchange{}, domainexchanges.ErrNotFound
	}
	return domainexchanges.Exchange{ID: id, Name: "OKX"}, nil
}

func TestMarketSearchRoute(t *testing.T) {
	const botToken = "test-token"
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := markethandlers.NewHandler(marketservice.NewService(activeExchangeStub{}, exchangedata.Stub{}), logger)
	router := NewRouter(logger, botToken, userhandlers.NewHandler(registerUserStub(nil), logger), nil, nil, nil, handler)

	for _, tc := range []struct {
		query string
		code  int
		body  string
	}{
		{"exchange_id=7&q=+btc+&limit=1", http.StatusOK,
			`{"items":[{"exchange_id":7,"external_id":"BTC-EUR","base_asset":"BTC","quote_asset":"EUR","display_name":"BTC/EUR","price":"77000"}],"next_page":2}`},
		{"exchange_id=7&q=btc&limit=1&page=2", http.StatusOK,
			`{"items":[{"exchange_id":7,"external_id":"BTC-USDT","base_asset":"BTC","quote_asset":"USDT","display_name":"BTC/USDT","price":"83781.50"}],"next_page":null}`},
		{"exchange_id=7&page=2147483647", http.StatusOK, `{"items":[],"next_page":null}`},
		{"exchange_id=7&q=unknown", http.StatusOK, `{"items":[],"next_page":null}`},
		{"exchange_id=99", http.StatusNotFound, `{"error":{"code":"EXCHANGE_NOT_FOUND","message":"Exchange not found"}}`},
		{"q=btc", http.StatusBadRequest, `{"error":{"code":"VALIDATION_ERROR","message":"Invalid market search parameters"}}`},
		{"exchange_id=7&q=btc&q=eth", http.StatusBadRequest, `{"error":{"code":"VALIDATION_ERROR","message":"Invalid market search parameters"}}`},
		{"exchange_id=7&sort=price", http.StatusBadRequest, `{"error":{"code":"VALIDATION_ERROR","message":"Invalid market search parameters"}}`},
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/markets?"+tc.query, nil)
		req.Header.Set("Authorization", signedTelegramHeader(botToken, 123))
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != tc.code || strings.TrimSpace(res.Body.String()) != tc.body {
			t.Errorf("%s: got %d %s, want %d %s", tc.query, res.Code, res.Body, tc.code, tc.body)
		}
	}
}
