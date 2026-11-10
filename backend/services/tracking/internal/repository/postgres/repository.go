package postgres

import (
	"context"
	"fmt"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"marketmind/contracts/events"
	"marketmind/tracking/internal/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) GetMarket(ctx context.Context, marketID int64) (domain.Market, error) {
	var market domain.Market
	err := r.pool.QueryRow(ctx, `
		SELECT m.id, e.name, e.is_active, m.external_id,
		       m.base_asset, m.quote_asset, m.price::text, m.updated_at
		FROM market_mind.markets AS m
		JOIN market_mind.exchanges AS e ON e.id = m.exchange_id
		WHERE m.id = $1
	`, marketID).Scan(
		&market.ID, &market.ExchangeName, &market.ExchangeActive, &market.ExternalID,
		&market.BaseAsset, &market.QuoteAsset, &market.Price, &market.UpdatedAt,
	)
	if err != nil {
		return domain.Market{}, fmt.Errorf("get market %d: %w", marketID, err)
	}
	return market, nil
}

func (r *Repository) GetActiveAlerts(ctx context.Context, marketID int64) ([]domain.Alert, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT a.id, a.type, s.market_id, u.telegram_user_id,
		       a.direction, a.reference_price::text, a.value::text,
		       a.is_notified, u.is_notification_enable
		FROM market_mind.alerts AS a
		JOIN market_mind.subscriptions AS s ON s.id = a.subscription_id
		JOIN market_mind.users AS u ON u.id = s.user_id
		WHERE s.market_id = $1 AND a.is_active = TRUE
		ORDER BY a.id
	`, marketID)
	if err != nil {
		return nil, fmt.Errorf("query active alerts for market %d: %w", marketID, err)
	}
	defer rows.Close()

	alerts := make([]domain.Alert, 0)
	for rows.Next() {
		var alert domain.Alert
		if err := rows.Scan(
			&alert.AlertID, &alert.Type, &alert.MarketID, &alert.TelegramUserID,
			&alert.Direction, &alert.ReferencePrice, &alert.Value,
			&alert.IsNotified, &alert.NotificationEnabled,
		); err != nil {
			return nil, fmt.Errorf("scan active alert for market %d: %w", marketID, err)
		}
		alerts = append(alerts, alert)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read active alerts for market %d: %w", marketID, err)
	}
	return alerts, nil
}

func (r *Repository) SetAlertNotified(ctx context.Context, alertID int64, notified bool) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE market_mind.alerts
		SET is_notified = $2
		WHERE id = $1
	`, alertID, notified)
	if err != nil {
		return fmt.Errorf("update alert %d notification flag: %w", alertID, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update alert %d notification flag: %w", alertID, pgx.ErrNoRows)
	}
	return nil
}

func (r *Repository) UpdateMarketPrice(ctx context.Context, price events.CheckedPrice) error {
	marketID, err := strconv.ParseInt(price.MarketID, 10, 64)
	if err != nil || marketID <= 0 {
		return fmt.Errorf("invalid market ID %q", price.MarketID)
	}
	if price.ReceivedAt.IsZero() {
		return fmt.Errorf("quote received time is required")
	}
	_, err = r.pool.Exec(ctx, `
		UPDATE market_mind.markets
		SET price = $2::text::numeric, updated_at = $3
		WHERE id = $1 AND updated_at < $3
	`, marketID, price.Price, price.ReceivedAt)
	if err != nil {
		return fmt.Errorf("update market %d price: %w", marketID, err)
	}
	return nil
}
