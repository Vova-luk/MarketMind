package subscriptions

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"marketmind/api/internal/domain/markets"
)

func (r *Repository) ListTrackedMarkets(ctx context.Context) ([]markets.TrackedMarket, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT e.name, m.id, m.external_id
		FROM market_mind.markets AS m
		JOIN market_mind.exchanges AS e ON e.id = m.exchange_id
		WHERE e.is_active
		  AND EXISTS (SELECT 1 FROM market_mind.subscriptions AS s WHERE s.market_id = m.id)
		ORDER BY e.name, m.id
	`)
	if err != nil {
		return nil, fmt.Errorf("query tracked markets: %w", err)
	}
	tracked, err := pgx.CollectRows(rows, pgx.RowToStructByPos[markets.TrackedMarket])
	if err != nil {
		return nil, fmt.Errorf("read tracked markets: %w", err)
	}
	return tracked, nil
}
