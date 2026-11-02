package migrator

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"marketmind/utils/migrations"
)

func Run(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) error {
	return run(ctx, pool, migrations.Files, logger)
}

func run(ctx context.Context, pool *pgxpool.Pool, files fs.FS, logger *slog.Logger) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close() // Closing the adapter leaves the original pool open.

	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("initialize migration lock: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, files,
		goose.WithSessionLocker(locker),
		goose.WithDisableGlobalRegistry(true),
		goose.WithSlog(logger),
	)
	if errors.Is(err, goose.ErrNoMigrations) {
		logger.Info("no migrations to apply")
		return nil
	}
	if err != nil {
		return fmt.Errorf("initialize migrator: %w", err)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	logger.Info("database migrations completed", "applied", len(results))
	return nil
}
