package migrator

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"marketmind/utils/migrations"
)

// Run applies pending migrations of the shared MarketMind database.
// The caller keeps ownership of pool; migrations run under a PostgreSQL lock.
func Run(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	return run(ctx, pool, migrations.Files, logger)
}

func run(ctx context.Context, pool *pgxpool.Pool, files fs.FS, logger *slog.Logger) error {
	if pool == nil {
		return fmt.Errorf("PostgreSQL pool is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close() // Closing the adapter leaves the original pool open.
	provider, err := newProvider(db, files, logger)
	if err != nil {
		return err
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	logger.Info("database migrations completed", "applied", len(results))
	return nil
}

func newProvider(db *sql.DB, files fs.FS, logger *slog.Logger) (*goose.Provider, error) {
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return nil, fmt.Errorf("initialize migration lock: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, files,
		goose.WithSessionLocker(locker),
		goose.WithDisableGlobalRegistry(true),
		goose.WithSlog(logger),
	)
	if err != nil {
		// An empty migration set is a packaging error, not a successful startup.
		return nil, fmt.Errorf("initialize migrator: %w", err)
	}
	return provider, nil
}
