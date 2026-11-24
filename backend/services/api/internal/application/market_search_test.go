package application

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"marketmind/api/internal/adapters/exchangedata"
	domainexchanges "marketmind/api/internal/domain/exchanges"
	markethandlers "marketmind/api/internal/handlers/markets"
	userhandlers "marketmind/api/internal/handlers/users"
	marketservice "marketmind/api/internal/service/markets"
	exchangedatapb "marketmind/gen/go/exchangedata"
)

type activeExchangeStub struct{}

type marketSearchClientFunc func(context.Context, *exchangedatapb.SearchMarketsRequest) (*exchangedatapb.SearchMarketsResponse, error)

func (f marketSearchClientFunc) SearchMarkets(ctx context.Context, req *exchangedatapb.SearchMarketsRequest, _ ...grpc.CallOption) (*exchangedatapb.SearchMarketsResponse, error) {
	return f(ctx, req)
}

func (activeExchangeStub) GetActive(_ context.Context, id int32) (domainexchanges.Exchange, error) {
	if id != 7 {
		return domainexchanges.Exchange{}, domainexchanges.ErrNotFound
	}
	return domainexchanges.Exchange{ID: id, Name: "OKX"}, nil
}

func TestMarketSearchRoute(t *testing.T) {
	const botToken = "test-token"
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	client := marketSearchClientFunc(func(_ context.Context, req *exchangedatapb.SearchMarketsRequest) (*exchangedatapb.SearchMarketsResponse, error) {
		if req.ExchangeName != "OKX" || req.Page == nil || req.Limit == nil {
			t.Fatalf("incorrect gRPC request: %v", req)
		}
		switch {
		case req.Query == "btc" && req.GetPage() == 1 && req.GetLimit() == 1:
			price, nextPage := "77000.126", int32(2)
			return &exchangedatapb.SearchMarketsResponse{NextPage: &nextPage, Items: []*exchangedatapb.Market{
				{ExternalId: "BTC-EUR", BaseAsset: "BTC", QuoteAsset: "EUR", Price: &price},
			}}, nil
		case req.Query == "btc" && req.GetPage() == 2 && req.GetLimit() == 1:
			return &exchangedatapb.SearchMarketsResponse{Items: []*exchangedatapb.Market{
				{ExternalId: "BTC-USDT", BaseAsset: "BTC", QuoteAsset: "USDT"},
			}}, nil
		case req.Query == "unknown" && req.GetPage() == 1 && req.GetLimit() == 20,
			req.Query == "" && req.GetPage() == 2147483647 && req.GetLimit() == 20:
			return &exchangedatapb.SearchMarketsResponse{}, nil
		case req.Query == "offline" && req.GetPage() == 1 && req.GetLimit() == 20:
			return nil, status.Error(codes.Unavailable, "upstream unavailable")
		default:
			t.Fatalf("unexpected gRPC request: %v", req)
			return nil, nil
		}
	})
	handler := markethandlers.NewHandler(marketservice.NewService(activeExchangeStub{}, exchangedata.NewCatalog(client, time.Second)), logger)
	router := NewRouter(logger, botToken, userhandlers.NewHandler(registerUserStub(nil), logger), nil, nil, nil, handler)

	for _, tc := range []struct {
		query string
		code  int
		body  string
	}{
		{"exchange_id=7&q=+btc+&limit=1", http.StatusOK,
			`{"items":[{"exchange_id":7,"external_id":"BTC-EUR","base_asset":"BTC","quote_asset":"EUR","display_name":"BTC/EUR","price":"77000.13","price_exact":"77000.126"}],"next_page":2}`},
		{"exchange_id=7&q=btc&limit=1&page=2", http.StatusOK,
			`{"items":[{"exchange_id":7,"external_id":"BTC-USDT","base_asset":"BTC","quote_asset":"USDT","display_name":"BTC/USDT","price":null,"price_exact":null}],"next_page":null}`},
		{"exchange_id=7&page=2147483647", http.StatusOK, `{"items":[],"next_page":null}`},
		{"exchange_id=7&q=unknown", http.StatusOK, `{"items":[],"next_page":null}`},
		{"exchange_id=7&q=offline", http.StatusServiceUnavailable,
			`{"error":{"code":"MARKET_CATALOG_UNAVAILABLE","message":"Market catalog is unavailable"}}`},
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
