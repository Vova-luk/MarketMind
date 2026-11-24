package utils

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	SubscriptionsPath = "/api/v1/subscriptions"
	BinanceExchangeID = 1
	BtcusdtExternalID = "BTCUSDT"
)

type StoredSubscription struct {
	ID         int64
	MarketID   int64
	TelegramID int64
}

// function creates a new test record in the database
func SeedBTCUSDTMarket(t *testing.T, ctx context.Context, db *pgx.Conn) int64 {
	t.Helper()
	var id int64
	err := db.QueryRow(ctx, `
		INSERT INTO market_mind.markets (exchange_id, external_id, base_asset, quote_asset, price)
		VALUES ($1, $2, 'BTC', 'USDT', 65000.50)
		ON CONFLICT (exchange_id, external_id) DO NOTHING
		RETURNING id
	`, BinanceExchangeID, BtcusdtExternalID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		// Reuse a market already present in the test database without deleting it at cleanup.
		err = db.QueryRow(ctx, `
			SELECT id FROM market_mind.markets
			WHERE exchange_id = $1 AND external_id = $2
		`, BinanceExchangeID, BtcusdtExternalID).Scan(&id)
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

func AssertStoredWatchlist(t *testing.T, ctx context.Context, db *pgx.Conn, telegramID, createdID int64, want []StoredSubscription) {
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
	got, err := pgx.CollectRows(rows, pgx.RowToStructByPos[StoredSubscription])
	if err != nil {
		t.Fatalf("read watchlist in PostgreSQL: %v", err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("watchlist in PostgreSQL: got %+v, want %+v", got, want)
	}
}
