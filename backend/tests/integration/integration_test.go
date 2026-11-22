package integration

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type SearchResponse struct {
	Items    []MarketOption `json:"items"`
	NextPage *int32         `json:"next_page"`
}

type MarketOption struct {
	ExchangeID  int32   `json:"exchange_id"`
	ExternalID  string  `json:"external_id"`
	BaseAsset   string  `json:"base_asset"`
	QuoteAsset  string  `json:"quote_asset"`
	DisplayName string  `json:"display_name"`
	Price       *string `json:"price"`
	PriceExact  *string `json:"price_exact"`
}

type SubscriptionResponse struct {
	ID           int64  `json:"subscription_id,string"`
	BaseAsset    string `json:"base_asset"`
	QuoteAsset   string `json:"quote_asset"`
	ExchangeName string `json:"exchange_name"`
}

func TestSmoke_IT01_WatchlistLifecycle(t *testing.T) {
	botToken := os.Getenv("MARKETMIND_TELEGRAM__BOT_TOKEN")
	if botToken == "" {
		t.Fatal("MARKETMIND_TELEGRAM__BOT_TOKEN must match the test API token")
	}
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("TEST_DATABASE_URL must point to the database used by the test API")
	}
	baseURL := strings.TrimRight(os.Getenv("TEST_API_URL"), "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8080"
	}

	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	db, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := db.Close(closeCtx); err != nil {
			t.Errorf("close test database: %v", err)
		}
	})

	randomID, err := rand.Int(rand.Reader, big.NewInt(1<<52))
	if err != nil {
		t.Fatalf("generate test user ID: %v", err)
	}
	telegramUserID := randomID.Int64() + 1
	var exists bool
	if err := db.QueryRow(ctx, `SELECT EXISTS (
        SELECT 1 FROM market_mind.users WHERE telegram_user_id=$1
    )`, telegramUserID).Scan(&exists); err != nil {
		t.Fatalf("check test user does not exist: %v", err)
	}
	if exists {
		t.Fatal("generated test user already exists; rerun with a new ID")
	}

	client := &http.Client{Timeout: 10 * time.Second}
	authorization := signedTelegramHeader(botToken, telegramUserID)
	var registered struct {
		TelegramUserID int64 `json:"telegram_user_id,string"`
	}

	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := db.Exec(cleanupCtx,
			"DELETE FROM market_mind.users WHERE telegram_user_id=$1", telegramUserID); err != nil {
			t.Errorf("clean up test user: %v", err)
		}
	})
	requestJSON(t, ctx, client, authorization, http.MethodPost,
		baseURL+"/api/v1/users/me", nil, http.StatusOK, &registered)
	if registered.TelegramUserID != telegramUserID {
		t.Fatalf("registered user ID: got %d, want %d", registered.TelegramUserID, telegramUserID)
	}

	var search SearchResponse
	requestJSON(t, ctx, client, authorization, http.MethodGet,
		baseURL+"/api/v1/markets?exchange_id=1&q=BTCUSDT", nil, http.StatusOK, &search)
	var market MarketOption
	for _, item := range search.Items {
		if item.ExchangeID == 1 && item.ExternalID == "BTCUSDT" {
			market = item
			break
		}
	}
	if market.ExternalID == "" {
		t.Fatalf("BTCUSDT on Binance not found: %+v", search.Items)
	}
	if market.PriceExact == nil || strings.TrimSpace(*market.PriceExact) == "" {
		t.Fatal("market search returned no exact price for BTCUSDT")
	}

	var subscription SubscriptionResponse
	requestJSON(t, ctx, client, authorization, http.MethodPost,
		baseURL+"/api/v1/subscriptions", market, http.StatusCreated, &subscription)
	if subscription.ID <= 0 || subscription.BaseAsset != market.BaseAsset ||
		subscription.QuoteAsset != market.QuoteAsset || subscription.ExchangeName != "Binance" {
		t.Fatalf("unexpected subscription: %+v", subscription)
	}

	var count int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM market_mind.subscriptions s
        JOIN market_mind.users u ON u.id=s.user_id
        JOIN market_mind.markets m ON m.id=s.market_id
        WHERE s.id=$1 AND u.telegram_user_id=$2 AND m.exchange_id=$3 AND m.external_id=$4`,
		subscription.ID, telegramUserID, market.ExchangeID, market.ExternalID).Scan(&count); err != nil {
		t.Fatalf("read subscription from database: %v", err)
	}
	if count != 1 {
		t.Fatalf("subscription rows in database: got %d, want 1", count)
	}

	var list struct {
		Items []SubscriptionResponse `json:"items"`
	}
	requestJSON(t, ctx, client, authorization, http.MethodGet,
		baseURL+"/api/v1/subscriptions", nil, http.StatusOK, &list)
	if len(list.Items) != 1 || list.Items[0].ID != subscription.ID {
		t.Fatalf("unexpected watchlist after addition: %+v", list.Items)
	}

	requestJSON(t, ctx, client, authorization, http.MethodDelete,
		fmt.Sprintf("%s/api/v1/subscriptions/%d", baseURL, subscription.ID), nil, http.StatusNoContent, nil)
	if err := db.QueryRow(ctx, `SELECT count(*) FROM market_mind.subscriptions
        WHERE id=$1 OR (user_id=(SELECT id FROM market_mind.users WHERE telegram_user_id=$2)
            AND market_id=(SELECT id FROM market_mind.markets WHERE exchange_id=$3 AND external_id=$4))`,
		subscription.ID, telegramUserID, market.ExchangeID, market.ExternalID).Scan(&count); err != nil {
		t.Fatalf("check deleted subscription in database: %v", err)
	}
	if count != 0 {
		t.Fatalf("subscription remains in database after deletion: %d rows", count)
	}

	list.Items = nil
	requestJSON(t, ctx, client, authorization, http.MethodGet,
		baseURL+"/api/v1/subscriptions", nil, http.StatusOK, &list)
	if len(list.Items) != 0 {
		t.Fatalf("watchlist is not empty after deletion: %+v", list.Items)
	}
}

func requestJSON(t *testing.T, ctx context.Context, client *http.Client, authorization, method, url string, payload any, wantStatus int, result any) {
	t.Helper()
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("encode %s %s request: %v", method, url, err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		t.Fatalf("create %s %s request: %v", method, url, err)
	}
	req.Header.Set("Authorization", authorization)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send %s %s request: %v", method, url, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s %s response: %v", method, url, err)
	}
	if resp.StatusCode != wantStatus {
		t.Fatalf("%s %s: status=%d, want=%d, body=%s", method, url, resp.StatusCode, wantStatus, data)
	}
	if result != nil {
		if err := json.Unmarshal(data, result); err != nil {
			t.Fatalf("decode %s %s response: %v; body=%s", method, url, err, data)
		}
	}
}
