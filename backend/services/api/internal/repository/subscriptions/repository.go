package subscriptions

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainsubscriptions "marketmind/api/internal/domain/subscriptions"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const subscriptionColumns = `s.id, m.base_asset, m.quote_asset, e.name,
	CASE WHEN e.is_active THEN m.price::text END`

const subscriptionJoins = `
	FROM market_mind.subscriptions AS s
	JOIN market_mind.users AS u ON u.id = s.user_id
	JOIN market_mind.markets AS m ON m.id = s.market_id
	JOIN market_mind.exchanges AS e ON e.id = m.exchange_id
`

func (r *Repository) List(ctx context.Context, telegramUserID int64, limit int, offset int64) ([]domainsubscriptions.Subscription, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+subscriptionColumns+subscriptionJoins+`
		WHERE u.telegram_user_id = $1
		ORDER BY s.id
		LIMIT $2 OFFSET $3
	`, telegramUserID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query user subscriptions: %w", err)
	}
	subscriptions, err := pgx.CollectRows(rows, pgx.RowToStructByPos[domainsubscriptions.Subscription])
	if err != nil {
		return nil, fmt.Errorf("read user subscriptions: %w", err)
	}
	return subscriptions, nil
}

func (r *Repository) Get(ctx context.Context, telegramUserID, subscriptionID int64) (domainsubscriptions.Detail, error) {
	return r.getDetail(ctx, telegramUserID, subscriptionID, nil, 0)
}

func (r *Repository) ListAlerts(ctx context.Context, telegramUserID, subscriptionID int64, limit int, offset int64) ([]domainsubscriptions.Alert, string, error) {
	detail, err := r.getDetail(ctx, telegramUserID, subscriptionID, &limit, offset)
	return detail.Alerts, detail.QuoteAsset, err
}

func (r *Repository) getDetail(ctx context.Context, telegramUserID, subscriptionID int64, alertLimit *int, alertOffset int64) (domainsubscriptions.Detail, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+subscriptionColumns+`, m.updated_at,
		       a.id, a.type, a.value::text, a.direction, a.reference_price::text,
		       (CASE a.type
		           WHEN 'price' THEN a.value
		           WHEN 'percent' THEN trim_scale(a.reference_price * (1 + a.value * 0.01))
		       END)::text AS target_price,
		       a.is_active
		`+subscriptionJoins+`
		LEFT JOIN LATERAL (
		    SELECT id, type, value, condition, reference_price, is_active
		    FROM market_mind.alerts
		    WHERE subscription_id = s.id
		    ORDER BY id
		    LIMIT $3 OFFSET $4
		) AS a ON TRUE
		WHERE u.telegram_user_id = $1 AND s.id = $2
		ORDER BY a.id
	`, telegramUserID, subscriptionID, alertLimit, alertOffset)
	if err != nil {
		return domainsubscriptions.Detail{}, fmt.Errorf("query subscription detail: %w", err)
	}
	defer rows.Close()

	var detail domainsubscriptions.Detail
	found := false
	for rows.Next() {
		var alertID *int64
		var alertType, value, direction, referencePrice, targetPrice *string
		var isActive *bool
		if err := rows.Scan(&detail.ID, &detail.BaseAsset, &detail.QuoteAsset,
			&detail.ExchangeName, &detail.Price, &detail.UpdatedAt,
			&alertID, &alertType, &value, &direction, &referencePrice, &targetPrice, &isActive); err != nil {
			return domainsubscriptions.Detail{}, fmt.Errorf("scan subscription detail: %w", err)
		}
		found = true
		if alertID != nil {
			if targetPrice == nil {
				return domainsubscriptions.Detail{}, fmt.Errorf("unknown type %q for alert %d", *alertType, *alertID)
			}
			detail.Alerts = append(detail.Alerts, domainsubscriptions.Alert{
				ID:             *alertID,
				Type:           *alertType,
				Value:          *value,
				Direction:      *direction,
				ReferencePrice: *referencePrice,
				TargetPrice:    *targetPrice,
				IsActive:       *isActive,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return domainsubscriptions.Detail{}, fmt.Errorf("read subscription detail: %w", err)
	}
	if !found {
		return domainsubscriptions.Detail{}, domainsubscriptions.ErrNotFound
	}
	return detail, nil
}

func (r *Repository) Delete(ctx context.Context, telegramUserID, subscriptionID int64) error {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM market_mind.subscriptions AS s
		USING market_mind.users AS u
		WHERE s.user_id = u.id AND u.telegram_user_id = $1 AND s.id = $2
	`, telegramUserID, subscriptionID)
	if err != nil {
		return fmt.Errorf("delete subscription: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domainsubscriptions.ErrNotFound
	}
	return nil
}
