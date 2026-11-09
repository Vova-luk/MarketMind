package migrator

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"marketmind/utils/migrations"
)

// The configured database is used only to create and drop a separate test DB.
func migrationTestPool(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("MIGRATOR_TEST_DSN")
	if dsn == "" {
		t.Skip("set MIGRATOR_TEST_DSN to run PostgreSQL migration integration tests")
	}
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	name := fmt.Sprintf("marketmind_migrations_%d", time.Now().UnixNano())
	quotedName := pgx.Identifier{name}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+quotedName); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := admin.Exec(cleanupCtx, "DROP DATABASE "+quotedName+" WITH (FORCE)"); err != nil {
			t.Errorf("drop test database: %v", err)
		}
	})
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = name
	cfg.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	return pool
}

func assertMarketTables(t *testing.T, ctx context.Context, pool *pgxpool.Pool, want []string) {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname = 'market_mind' ORDER BY tablename`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected tables: got %v, want %v", got, want)
	}
}

func TestMigrationLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pool := migrationTestPool(t, ctx)
	logger := testLogger()
	if err := Run(ctx, pool, logger); err != nil {
		t.Fatal(err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("migrator closed the application pool: %v", err)
	}
	tables := []string{"alerts", "exchanges", "markets", "subscriptions", "users"}
	assertMarketTables(t, ctx, pool, tables)
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	provider, err := newProvider(db, migrations.Files, logger)
	if err != nil {
		t.Fatal(err)
	}
	if version, err := provider.GetDBVersion(ctx); err != nil || version != 6 {
		t.Fatalf("expected version 6, got %d: %v", version, err)
	}
	if results, err := provider.Up(ctx); err != nil || len(results) != 0 {
		t.Fatalf("second Up must be a no-op: applied %d, error %v", len(results), err)
	}
	assertSeededExchanges(t, ctx, pool)
	t.Log("Up and repeated Up succeeded")

	var marketID, userID, subscriptionID int64
	const price = "0.123456789012345678"
	if err := pool.QueryRow(ctx, `INSERT INTO market_mind.markets (exchange_id, external_id, base_asset, quote_asset, price) VALUES (1, 'BTCUSDT', 'BTC', 'USDT', $1) RETURNING id`, price).Scan(&marketID); err != nil {
		t.Fatal(err)
	}
	var storedPrice string
	if err := pool.QueryRow(ctx, `SELECT price::text FROM market_mind.markets WHERE id=$1`, marketID).Scan(&storedPrice); err != nil || storedPrice != price {
		t.Fatalf("decimal price lost precision: %q, error %v", storedPrice, err)
	}
	var createdAt, updatedAt time.Time
	if err := pool.QueryRow(ctx, `SELECT created_at, updated_at FROM market_mind.markets WHERE id=$1`, marketID).
		Scan(&createdAt, &updatedAt); err != nil || !createdAt.Equal(updatedAt) {
		t.Fatalf("initial market timestamps: created %v, updated %v, error %v", createdAt, updatedAt, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE market_mind.markets SET updated_at = '2000-01-01T00:00:00Z' WHERE id=$1`, marketID); err != nil {
		t.Fatal(err)
	}
	var createdAfter time.Time
	if err := pool.QueryRow(ctx, `UPDATE market_mind.markets SET price = 1.25, updated_at = statement_timestamp()
		WHERE id=$1 RETURNING created_at, updated_at`, marketID).
		Scan(&createdAfter, &updatedAt); err != nil || !createdAfter.Equal(createdAt) || updatedAt.Before(createdAt) {
		t.Fatalf("market timestamps after price write: created %v, updated %v, error %v", createdAfter, updatedAt, err)
	}
	// One coin may have several quote assets on one exchange. External IDs are
	// exchange-scoped and must retain their original spelling.
	if _, err := pool.Exec(ctx, `INSERT INTO market_mind.markets
		(exchange_id, external_id, base_asset, quote_asset, price)
		VALUES (1, 'BTCEUR', 'BTC', 'EUR', 77000),
		       (2, 'BTCUSDT', 'BTC', 'USDT', 83781),
		       (3, 'BTC-USDT', 'BTC', 'USDT', 83781)`); err != nil {
		t.Fatalf("multiple quote assets and exchange-specific symbols: %v", err)
	}
	var symbol, baseAsset, quoteAsset string
	if err := pool.QueryRow(ctx, `SELECT external_id, base_asset, quote_asset FROM market_mind.markets
		WHERE exchange_id=3`).Scan(&symbol, &baseAsset, &quoteAsset); err != nil || symbol != "BTC-USDT" || baseAsset != "BTC" || quoteAsset != "USDT" {
		t.Fatalf("instrument metadata changed: %q %q %q, error %v", symbol, baseAsset, quoteAsset, err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO market_mind.users (telegram_user_id) VALUES (12345) RETURNING id`).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO market_mind.subscriptions (user_id, market_id) VALUES ($1, $2) RETURNING id`, userID, marketID).Scan(&subscriptionID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO market_mind.alerts (type, subscription_id, value, condition, reference_price)
		VALUES ('percent', $1, 2.5, 'gte', $2)`, subscriptionID, price); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, query, code string
		args              []any
	}{
		{"duplicate Telegram user", `INSERT INTO market_mind.users (telegram_user_id) VALUES (12345)`, "23505", nil},
		{"duplicate subscription", `INSERT INTO market_mind.subscriptions (user_id, market_id) VALUES ($1, $2)`, "23505", []any{userID, marketID}},
		{"duplicate external ID", `INSERT INTO market_mind.markets (exchange_id, external_id, base_asset, quote_asset, price) VALUES (1, 'BTCUSDT', 'ETH', 'EUR', 1)`, "23505", nil},
		{"duplicate pair with another external ID", `INSERT INTO market_mind.markets (exchange_id, external_id, base_asset, quote_asset, price) VALUES (1, 'BTC-USDT', 'BTC', 'USDT', 1)`, "23505", nil},
		{"unknown exchange", `INSERT INTO market_mind.markets (exchange_id, external_id, base_asset, quote_asset, price) VALUES (999, 'BTCUSDT', 'BTC', 'USDT', 1)`, "23503", nil},
		{"missing external ID", `UPDATE market_mind.markets SET external_id=NULL WHERE id=$1`, "23502", []any{marketID}},
		{"missing base asset", `UPDATE market_mind.markets SET base_asset=NULL WHERE id=$1`, "23502", []any{marketID}},
		{"missing quote asset", `UPDATE market_mind.markets SET quote_asset=NULL WHERE id=$1`, "23502", []any{marketID}},
	} {
		_, err := pool.Exec(ctx, tc.query, tc.args...)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != tc.code {
			t.Fatalf("%s: expected SQLSTATE %s, got %v", tc.name, tc.code, err)
		}
	}
	if _, err := pool.Exec(ctx, `DELETE FROM market_mind.users WHERE id=$1`, userID); err != nil {
		t.Fatal(err)
	}
	var dependents int
	if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM market_mind.subscriptions) + (SELECT count(*) FROM market_mind.alerts)`).Scan(&dependents); err != nil || dependents != 0 {
		t.Fatalf("cascade deletion failed: rows %d, error %v", dependents, err)
	}
	t.Log("decimal values, unique constraints, foreign keys and cascades succeeded")

	if _, err := provider.Down(ctx); err != nil {
		t.Fatal(err)
	}
	assertMarketTables(t, ctx, pool, tables[1:])
	assertSeededExchanges(t, ctx, pool)
	if results, err := provider.Up(ctx); err != nil || len(results) != 1 {
		t.Fatalf("reapply last migration: applied %d, error %v", len(results), err)
	}
	assertSeededExchanges(t, ctx, pool)
	if results, err := provider.DownTo(ctx, 0); err != nil || len(results) != 6 {
		t.Fatalf("full rollback: reverted %d, error %v", len(results), err)
	}
	assertMarketTables(t, ctx, pool, nil)
	if version, err := provider.GetDBVersion(ctx); err != nil || version != 0 {
		t.Fatalf("expected version 0 after rollback, got %d: %v", version, err)
	}
	if results, err := provider.Up(ctx); err != nil || len(results) != 6 {
		t.Fatalf("Up after rollback: applied %d, error %v", len(results), err)
	}
	assertMarketTables(t, ctx, pool, tables)
	assertSeededExchanges(t, ctx, pool)
	t.Log("single Down, full Down and Up after rollback succeeded")

	files := fstest.MapFS{}
	entries, err := fs.ReadDir(migrations.Files, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		data, err := migrations.Files.ReadFile(entry.Name())
		if err != nil {
			t.Fatal(err)
		}
		files[entry.Name()] = &fstest.MapFile{Data: data}
	}
	files["007_fail.sql"] = &fstest.MapFile{Data: []byte("-- +goose Up\nCREATE TABLE market_mind.failed_migration (id BIGINT);\nSELECT * FROM market_mind.missing_table;\n")}
	if err := run(ctx, pool, files, logger); err == nil {
		t.Fatal("invalid migration unexpectedly succeeded")
	}
	var rolledBack bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('market_mind.failed_migration') IS NULL`).Scan(&rolledBack); err != nil || !rolledBack {
		t.Fatalf("failed migration was not rolled back: %v", err)
	}
	if version, err := provider.GetDBVersion(ctx); err != nil || version != 6 {
		t.Fatalf("failed migration changed the version: %d, error %v", version, err)
	}
	t.Log("failed migration rolled back atomically")
}

func assertSeededExchanges(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT id, name, is_active FROM market_mind.exchanges ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	names := []string{"Binance", "Bybit", "OKX"}
	i := 0
	for rows.Next() {
		var id int
		var name string
		var active bool
		if err := rows.Scan(&id, &name, &active); err != nil {
			t.Fatal(err)
		}
		if i >= len(names) {
			t.Fatal("unexpected extra exchange")
		}
		if id != i+1 || name != names[i] || !active {
			t.Fatalf("unexpected seeded exchange: id=%d, name=%q, active=%v", id, name, active)
		}
		i++
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if i != len(names) {
		t.Fatalf("expected %d exchanges, got %d", len(names), i)
	}
}
