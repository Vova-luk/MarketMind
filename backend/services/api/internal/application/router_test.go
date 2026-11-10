package application

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	domainsubscriptions "marketmind/api/internal/domain/subscriptions"
	domainusers "marketmind/api/internal/domain/users"
	alerthandlers "marketmind/api/internal/handlers/alerts"
	subscriptionhandlers "marketmind/api/internal/handlers/subscriptions"
	userhandlers "marketmind/api/internal/handlers/users"
)

type registerUserStub func(context.Context, domainusers.User) error

func (s registerUserStub) Register(ctx context.Context, user domainusers.User) error {
	return s(ctx, user)
}

func (s registerUserStub) Exists(context.Context, int64) (bool, error) {
	return true, nil
}

func TestRegisterUserRouteUsesSignedTelegramID(t *testing.T) {
	const botToken = "test-token"
	const telegramUserID int64 = 1234567890123
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	repository := registerUserStub(func(_ context.Context, user domainusers.User) error {
		if user.TelegramUserID != telegramUserID {
			t.Fatalf("repository received Telegram ID %d, want %d", user.TelegramUserID, telegramUserID)
		}
		return nil
	})
	router := NewRouter(logger, botToken, userhandlers.NewHandler(repository, logger), nil, nil, nil, nil)

	userID := strconv.FormatInt(telegramUserID, 10)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/users/me", strings.NewReader(`{"telegram_user_id":"999"}`))
	req.Header.Set("Authorization", signedTelegramHeader(botToken, telegramUserID))
	res := httptest.NewRecorder()
	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200. Body: %s", res.Code, res.Body)
	}
	var body map[string]any
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 || body["telegram_user_id"] != userID {
		t.Fatalf("unexpected response: %s", res.Body)
	}
}

func signedTelegramHeader(botToken string, telegramUserID int64) string {
	fields := url.Values{
		"auth_date": {strconv.FormatInt(time.Now().Unix(), 10)},
		"user":      {`{"id":` + strconv.FormatInt(telegramUserID, 10) + `}`},
	}
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	_, _ = secret.Write([]byte(botToken))
	signature := hmac.New(sha256.New, secret.Sum(nil))
	_, _ = signature.Write([]byte("auth_date=" + fields.Get("auth_date") + "\nuser=" + fields.Get("user")))
	fields.Set("hash", hex.EncodeToString(signature.Sum(nil)))
	return "tma " + fields.Encode()
}

type subscriptionReaderStub struct {
	domainsubscriptions.Repository
	get        func(context.Context, int64, int64) (domainsubscriptions.Detail, error)
	list       func(context.Context, int64, int, int64) ([]domainsubscriptions.Subscription, error)
	listAlerts func(context.Context, int64, int64, int, int64) ([]domainsubscriptions.Alert, string, error)
}

func (s subscriptionReaderStub) Get(ctx context.Context, telegramUserID, subscriptionID int64) (domainsubscriptions.Detail, error) {
	return s.get(ctx, telegramUserID, subscriptionID)
}

func (s subscriptionReaderStub) List(ctx context.Context, telegramUserID int64, limit int, offset int64) ([]domainsubscriptions.Subscription, error) {
	return s.list(ctx, telegramUserID, limit, offset)
}

func (s subscriptionReaderStub) ListAlerts(ctx context.Context, telegramUserID, subscriptionID int64, limit int, offset int64) ([]domainsubscriptions.Alert, string, error) {
	return s.listAlerts(ctx, telegramUserID, subscriptionID, limit, offset)
}

func TestSubscriptionDetailRoute(t *testing.T) {
	const botToken = "test-token"
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	reader := subscriptionReaderStub{get: func(_ context.Context, telegramUserID, subscriptionID int64) (domainsubscriptions.Detail, error) {
		if telegramUserID != 123 || subscriptionID != 18 {
			t.Fatalf("unexpected subscription lookup: Telegram user %d, subscription %d", telegramUserID, subscriptionID)
		}
		price := "83781.00000000000001"
		return domainsubscriptions.Detail{
			Subscription: domainsubscriptions.Subscription{
				ID: 18, BaseAsset: "BTC", QuoteAsset: "USDT", ExchangeName: "Binance", Price: &price,
			},
			UpdatedAt: time.Date(2026, 11, 9, 11, 30, 0, 0, time.FixedZone("UTC+3", 3*60*60)),
			Alerts:    []domainsubscriptions.Alert{{ID: 9, Type: "percent", Value: "5", Direction: "gte", ReferencePrice: "83781", TargetPrice: "87970.05", IsActive: true}},
		}, nil
	}}
	router := NewRouter(logger, botToken, userhandlers.NewHandler(registerUserStub(nil), logger), nil,
		subscriptionhandlers.NewHandler(reader, nil, logger), alerthandlers.NewHandler(nil, reader, logger), nil)

	for _, tc := range []struct {
		path   string
		status int
		body   string
	}{
		{"/api/v1/subscriptions/18", http.StatusOK,
			`{"subscription_id":"18","base_asset":"BTC","quote_asset":"USDT","exchange_name":"Binance","price":"83781.00000000000001","updated_at":"2026-11-09T08:30:00Z","alerts":[{"alert_id":"9","type":"percent","value":"5","direction":"gte","reference_price":"83781","target_price":"87970.05","is_active":true}]}`},
		{"/api/v1/subscriptions/18/alerts", http.StatusOK,
			`{"items":[{"alert_id":"9","type":"percent","value":"5","direction":"gte","reference_price":"83781","target_price":"87970.05","is_active":true,"quote_asset":"USDT"}]}`},
		{"/api/v1/subscriptions/018", http.StatusBadRequest,
			`{"error":{"code":"VALIDATION_ERROR","message":"Invalid subscription ID"}}`},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		req.Header.Set("Authorization", signedTelegramHeader(botToken, 123))
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		if res.Code != tc.status || strings.TrimSpace(res.Body.String()) != tc.body {
			t.Fatalf("%s returned status %d and body %s", tc.path, res.Code, res.Body)
		}
	}
}

func TestListPagination(t *testing.T) {
	const botToken = "test-token"
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, tc := range []struct {
		query     string
		limit     int
		offset    int64
		fetched   int
		wantCount int
		wantNext  int
	}{
		{query: "", limit: 11, fetched: 11, wantCount: 10, wantNext: 2},
		{query: "?page=2&page_size=3", limit: 4, offset: 3, fetched: 2, wantCount: 2},
		{query: "?page=3", limit: 11, offset: 20},
		{query: "?page_size=2", limit: 3, fetched: 3, wantCount: 2, wantNext: 2},
		{query: "?page_size=0"},
		{query: "?page_size=101"},
		{query: "?page=1&page=2"},
		{query: "?limit=2"},
	} {
		checkPage := func(uid int64, limit int, offset int64) {
			t.Helper()
			if tc.limit == 0 || uid != 123 || limit != tc.limit || offset != tc.offset {
				t.Fatalf("%q: unexpected repository arguments: user=%d limit=%d offset=%d", tc.query, uid, limit, offset)
			}
		}
		reader := subscriptionReaderStub{
			list: func(_ context.Context, uid int64, limit int, offset int64) ([]domainsubscriptions.Subscription, error) {
				checkPage(uid, limit, offset)
				return make([]domainsubscriptions.Subscription, tc.fetched), nil
			},
			listAlerts: func(_ context.Context, uid, subscriptionID int64, limit int, offset int64) ([]domainsubscriptions.Alert, string, error) {
				checkPage(uid, limit, offset)
				if subscriptionID != 18 {
					t.Fatalf("unexpected subscription ID %d", subscriptionID)
				}
				return make([]domainsubscriptions.Alert, tc.fetched), "USDT", nil
			},
		}
		router := NewRouter(logger, botToken, userhandlers.NewHandler(registerUserStub(nil), logger), nil,
			subscriptionhandlers.NewHandler(reader, nil, logger), alerthandlers.NewHandler(nil, reader, logger), nil)
		for _, path := range []string{"/api/v1/subscriptions", "/api/v1/subscriptions/18/alerts"} {
			req := httptest.NewRequest(http.MethodGet, path+tc.query, nil)
			req.Header.Set("Authorization", signedTelegramHeader(botToken, 123))
			res := httptest.NewRecorder()
			router.ServeHTTP(res, req)
			if tc.limit == 0 {
				if res.Code != http.StatusBadRequest {
					t.Fatalf("%s: got %d, want 400", path+tc.query, res.Code)
				}
				continue
			}
			var body struct {
				Items    []json.RawMessage `json:"items"`
				NextPage *int              `json:"next_page"`
			}
			if res.Code != http.StatusOK || json.Unmarshal(res.Body.Bytes(), &body) != nil ||
				body.Items == nil || len(body.Items) != tc.wantCount ||
				(tc.wantNext == 0 && body.NextPage != nil) ||
				(tc.wantNext != 0 && (body.NextPage == nil || *body.NextPage != tc.wantNext)) {
				t.Fatalf("%s: unexpected %d %s", path+tc.query, res.Code, res.Body)
			}
		}
	}
}
