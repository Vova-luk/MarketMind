package postgres

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	Host           string        `yaml:"host"`
	Port           int           `yaml:"port"`
	User           string        `yaml:"user"`
	Password       string        `yaml:"password"`
	Database       string        `yaml:"database"`
	SSLMode        string        `yaml:"sslmode"`
	MaxConns       int32         `yaml:"max_conns"`
	ConnectTimeout time.Duration `yaml:"connect_timeout"`
}

func (c Config) Validate() error {
	if c.Host == "" || c.User == "" || c.Password == "" || c.Database == "" {
		return errors.New("database host, user, password and database are required")
	}
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("database.port must be between 1 and 65535")
	}
	if c.MaxConns < 1 || c.ConnectTimeout <= 0 {
		return errors.New("database max_conns and connect_timeout must be positive")
	}
	return nil
}

func Connect(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	connectionURL := url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		User:   url.UserPassword(cfg.User, cfg.Password),
		Path:   "/" + cfg.Database,
	}
	query := connectionURL.Query()
	query.Set("sslmode", cfg.SSLMode)
	connectionURL.RawQuery = query.Encode()
	poolConfig, err := pgxpool.ParseConfig(connectionURL.String())
	if err != nil {

		return nil, errors.New("invalid PostgreSQL connection configuration")
	}
	poolConfig.MaxConns = cfg.MaxConns
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
