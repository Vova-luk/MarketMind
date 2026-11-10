package database

import (
	"context"
	"fmt"
	"net"
	"net/url"

	"github.com/jackc/pgx/v5/pgxpool"

	"marketmind/tracking/internal/config"
)

func Connect(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	address := url.URL{
		Scheme: "postgres", Host: net.JoinHostPort(cfg.Host, cfg.Port),
		User: url.UserPassword(cfg.User, cfg.Password), Path: "/" + cfg.Name,
	}
	query := address.Query()
	query.Set("sslmode", cfg.SSLMode)
	address.RawQuery = query.Encode()
	poolConfig, err := pgxpool.ParseConfig(address.String())
	if err != nil {
		return nil, fmt.Errorf("invalid PostgreSQL connection configuration")
	}
	poolConfig.MaxConns = int32(cfg.PoolMaxConns)
	poolConfig.MinConns = int32(cfg.PoolMinConns)
	poolConfig.ConnConfig.ConnectTimeout = cfg.ConnectTimeout
	connectCtx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(connectCtx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL pool: %w", err)
	}
	if err := pool.Ping(connectCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}
	return pool, nil
}
