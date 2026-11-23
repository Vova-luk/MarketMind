package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func MarketPrice(t testing.TB, ctx context.Context, db *pgx.Conn, marketID int64) (string, time.Time) {
	t.Helper()
	var price string
	var updatedAt time.Time
	if err := db.QueryRow(ctx, `
		SELECT price::text, updated_at FROM market_mind.markets WHERE id = $1
	`, marketID).Scan(&price, &updatedAt); err != nil {
		t.Fatalf("read market %d price: %v", marketID, err)
	}
	return price, updatedAt
}
