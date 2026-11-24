//go:build integration

package integration

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"marketmind-tests/internal/testutil"
)

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
	market := seedMarket(t, ctx, env.DB, spotMarket(binance, "BTC", "65000.50"))
	user := env.RegisterUser(t, ctx)

	created := addSubscription(t, ctx, user, market)
	want := subscriptionResponse{
		ID: created.ID, BaseAsset: "BTC", QuoteAsset: "USDT", ExchangeName: "Binance",
	}
	if created != want {
		t.Fatalf("created subscription: got %+v, want %+v", created, want)
	}

	var afterAdd watchlistResponse
	user.API.RequestJSON(t, ctx, http.MethodGet, subscriptionsPath, nil, http.StatusOK, &afterAdd)
	if !slices.Equal(afterAdd.Items, []subscriptionResponse{want}) {
		t.Fatalf("watchlist after add: got %+v, want [%+v]", afterAdd.Items, want)
	}
	assertStoredWatchlist(t, ctx, env.DB, user.TelegramID, created.ID,
		[]storedSubscription{{ID: created.ID, MarketID: market.ID, TelegramID: user.TelegramID}})

	user.API.RequestJSON(t, ctx, http.MethodDelete, fmt.Sprintf("%s/%d", subscriptionsPath, created.ID),
		nil, http.StatusNoContent, nil)

	var afterDelete watchlistResponse
	user.API.RequestJSON(t, ctx, http.MethodGet, subscriptionsPath, nil, http.StatusOK, &afterDelete)
	if afterDelete.Items == nil || len(afterDelete.Items) != 0 {
		t.Fatalf("watchlist after delete: got %#v, want an empty items array", afterDelete.Items)
	}
	assertStoredWatchlist(t, ctx, env.DB, user.TelegramID, created.ID, nil)
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
