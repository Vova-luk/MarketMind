//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"marketmind-tests/internal/testutil"
)

const (
	subscriptionsPath = "/api/v1/subscriptions"
	binanceExchangeID = 1
	btcusdtExternalID = "BTCUSDT"
)

type subscriptionResponse struct {
	ID           int64  `json:"subscription_id,string"`
	BaseAsset    string `json:"base_asset"`
	QuoteAsset   string `json:"quote_asset"`
	ExchangeName string `json:"exchange_name"`
}

type watchlistResponse struct {
	Items []subscriptionResponse `json:"items"`
}

type storedSubscription struct {
	ID         int64
	MarketID   int64
	TelegramID int64
}

func TestIT01_WatchlistLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	env := testutil.New(t, ctx)
	marketID := seedBTCUSDTMarket(t, ctx, env.DB)
	user := env.RegisterUser(t, ctx)

	var created subscriptionResponse
	user.API.RequestJSON(t, ctx, http.MethodPost, subscriptionsPath, map[string]any{
		"exchange_id": binanceExchangeID,
		"external_id": btcusdtExternalID,
	}, http.StatusCreated, &created)
	want := subscriptionResponse{
		ID: created.ID, BaseAsset: "BTC", QuoteAsset: "USDT", ExchangeName: "Binance",
	}
	if created.ID <= 0 || created != want {
		t.Fatalf("created subscription: got %+v, want a positive ID and %+v", created, want)
	}

	var afterAdd watchlistResponse
	user.API.RequestJSON(t, ctx, http.MethodGet, subscriptionsPath, nil, http.StatusOK, &afterAdd)
	if !slices.Equal(afterAdd.Items, []subscriptionResponse{want}) {
		t.Fatalf("watchlist after add: got %+v, want [%+v]", afterAdd.Items, want)
	}
	assertStoredWatchlist(t, ctx, env.DB, user.TelegramID, created.ID,
		[]storedSubscription{{ID: created.ID, MarketID: marketID, TelegramID: user.TelegramID}})

	user.API.RequestJSON(t, ctx, http.MethodDelete, fmt.Sprintf("%s/%d", subscriptionsPath, created.ID),
		nil, http.StatusNoContent, nil)

	var afterDelete watchlistResponse
	user.API.RequestJSON(t, ctx, http.MethodGet, subscriptionsPath, nil, http.StatusOK, &afterDelete)
	if afterDelete.Items == nil || len(afterDelete.Items) != 0 {
		t.Fatalf("watchlist after delete: got %#v, want an empty items array", afterDelete.Items)
	}
	assertStoredWatchlist(t, ctx, env.DB, user.TelegramID, created.ID, nil)
}

func seedBTCUSDTMarket(t *testing.T, ctx context.Context, db *pgx.Conn) int64 {
	t.Helper()
	var id int64
	err := db.QueryRow(ctx, `
		INSERT INTO market_mind.markets (exchange_id, external_id, base_asset, quote_asset, price)
		VALUES ($1, $2, 'BTC', 'USDT', 65000.50)
		ON CONFLICT (exchange_id, external_id) DO NOTHING
		RETURNING id
	`, binanceExchangeID, btcusdtExternalID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		// Reuse a market already present in the test database without deleting it at cleanup.
		err = db.QueryRow(ctx, `
			SELECT id FROM market_mind.markets
			WHERE exchange_id = $1 AND external_id = $2
		`, binanceExchangeID, btcusdtExternalID).Scan(&id)
		if err != nil {
			t.Fatalf("look up BTC/USDT fixture: %v", err)
		}
		return id
	}
	if err != nil {
		t.Fatalf("insert BTC/USDT fixture: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := db.Exec(ctx, "DELETE FROM market_mind.markets WHERE id = $1", id); err != nil {
			t.Errorf("delete BTC/USDT fixture: %v", err)
		}
	})
	return id
}

func assertStoredWatchlist(t *testing.T, ctx context.Context, db *pgx.Conn, telegramID, createdID int64, want []storedSubscription) {
	t.Helper()
	// Also check the returned ID, even if the API stored it under the wrong user.
	rows, err := db.Query(ctx, `
		SELECT s.id, s.market_id, u.telegram_user_id
		FROM market_mind.subscriptions AS s
		JOIN market_mind.users AS u ON u.id = s.user_id
		WHERE u.telegram_user_id = $1 OR s.id = $2
	`, telegramID, createdID)
	if err != nil {
		t.Fatalf("query watchlist in PostgreSQL: %v", err)
	}
	got, err := pgx.CollectRows(rows, pgx.RowToStructByPos[storedSubscription])
	if err != nil {
		t.Fatalf("read watchlist in PostgreSQL: %v", err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("watchlist in PostgreSQL: got %+v, want %+v", got, want)
	}
}
