package exchanges

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainexchanges "marketmind/api/internal/domain/exchanges"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) ListActive(ctx context.Context) ([]domainexchanges.Exchange, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name
		FROM market_mind.exchanges
		WHERE is_active
		ORDER BY name
	`)
	if err != nil {
		return nil, fmt.Errorf("query active exchanges: %w", err)
	}
	exchanges, err := pgx.CollectRows(rows, pgx.RowToStructByPos[domainexchanges.Exchange])
	if err != nil {
		return nil, fmt.Errorf("read active exchanges: %w", err)
	}
	return exchanges, nil
}
