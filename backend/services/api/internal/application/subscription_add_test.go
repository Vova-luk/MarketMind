package application

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	domain "marketmind/api/internal/domain/subscriptions"
	subscriptionhandlers "marketmind/api/internal/handlers/subscriptions"
	userhandlers "marketmind/api/internal/handlers/users"
	subscriptionservice "marketmind/api/internal/service/subscriptions"
)

type subscriptionAddStore struct {
	domain.AddTransaction
	market domain.Market
	userID int64
	owners map[int64]int64
}

func (s *subscriptionAddStore) BeginAdd(_ context.Context, userID int64) (domain.AddTransaction, error) {
	s.userID = userID
	return s, nil
}

func (s *subscriptionAddStore) GetExchange(context.Context, int32) (domain.Exchange, error) {
	return domain.Exchange{Name: "Binance", Active: true}, nil
}

func (s *subscriptionAddStore) FindMarket(context.Context, domain.MarketKey, domain.Exchange) (domain.Market, error) {
	market := s.market
	market.Subscription.ID = s.owners[s.userID]
	return market, nil
}

func (s *subscriptionAddStore) CountSubscriptions(context.Context) (int, error) {
	return 0, nil
}

func (s *subscriptionAddStore) InsertMarket(_ context.Context, _ domain.MarketKey, exchange domain.Exchange, seed domain.MarketSeed) (domain.Market, error) {
	s.market = domain.Market{
		MarketID: 42,
		Subscription: domain.Subscription{
			BaseAsset: seed.BaseAsset, QuoteAsset: seed.QuoteAsset, Price: &seed.Price, ExchangeName: exchange.Name,
		},
	}
	return s.market, nil
}

func (s *subscriptionAddStore) InsertSubscription(_ context.Context, _ int64) (int64, error) {
	id := int64(len(s.owners) + 18)
	s.owners[s.userID] = id
	return id, nil
}

func (s *subscriptionAddStore) Commit(context.Context) error   { return nil }
func (s *subscriptionAddStore) Rollback(context.Context) error { return nil }

func TestAddSubscriptionAcceptsSearchResultAndReusesMarket(t *testing.T) {
	const botToken = "test-token"
	const quotedPrice = "83781.123456789012345678"
	store := &subscriptionAddStore{owners: make(map[int64]int64)}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := subscriptionhandlers.NewHandler(nil, subscriptionservice.NewService(store, 50), logger)
	router := NewRouter(logger, botToken, userhandlers.NewHandler(registerUserStub(nil), logger), nil, handler, nil)
	post := func(userID int64, body map[string]any, status int, wantBody string) *httptest.ResponseRecorder {
		t.Helper()
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/subscriptions", bytes.NewReader(data))
		req.Header.Set("Authorization", signedTelegramHeader(botToken, userID))
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != status || !strings.Contains(res.Body.String(), wantBody) {
			t.Fatalf("got status %d and body %s, want status %d and %q", res.Code, res.Body, status, wantBody)
		}
		return res
	}

	body := map[string]any{"exchange_id": 1, "external_id": "BTCUSDT"}
	post(123, body, http.StatusBadRequest, "VALIDATION_ERROR")
	body["base_asset"] = "BTC"
	body["quote_asset"] = "USDT"
	body["display_name"] = "BTC/USDT"
	body["price"] = nil
	post(123, body, http.StatusBadRequest, "VALIDATION_ERROR")
	body["price"] = quotedPrice
	res := post(123, body, http.StatusCreated, quotedPrice)
	if res.Header().Get("Location") != "/api/v1/subscriptions/18" {
		t.Fatalf("unexpected Location: %q", res.Header().Get("Location"))
	}

	latestPrice := "84000"
	store.market.Price = &latestPrice
	body["price"] = "1"
	body["base_asset"] = "ETH"
	post(123, body, http.StatusOK, `"subscription_id":"18"`)
	post(456, body, http.StatusCreated, `"price":"84000"`)
	if store.market.Price != &latestPrice || store.market.BaseAsset != "BTC" || len(store.owners) != 2 {
		t.Fatal("reusing the market must preserve its data and one subscription per user")
	}
}
