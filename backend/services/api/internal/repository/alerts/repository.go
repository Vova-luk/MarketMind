package alerts

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domain "marketmind/api/internal/domain/alerts"
	"marketmind/api/internal/domain/subscriptions"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) BeginSubscription(ctx context.Context, telegramUserID, subscriptionID int64) (domain.Subscription, domain.Transaction, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return domain.Subscription{}, nil, fmt.Errorf("begin alert transaction: %w", err)
	}
	transaction := subscriptionTx{tx: tx, subscriptionID: subscriptionID}
	var subscription domain.Subscription
	// Lock the user for the per-user limit and the subscription against cascade deletion.
	err = tx.QueryRow(ctx, `
		SELECT u.id, m.price::text, m.quote_asset, e.is_active
		FROM market_mind.users AS u
		JOIN market_mind.subscriptions AS s ON s.user_id = u.id
		JOIN market_mind.markets AS m ON m.id = s.market_id
		JOIN market_mind.exchanges AS e ON e.id = m.exchange_id
		WHERE u.telegram_user_id = $1 AND s.id = $2
		FOR NO KEY UPDATE OF u, s
	`, telegramUserID, subscriptionID).Scan(&transaction.userID, &subscription.Price, &subscription.QuoteAsset, &subscription.ExchangeActive)
	if err != nil {
		_ = tx.Rollback(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Subscription{}, nil, subscriptions.ErrNotFound
		}
		return domain.Subscription{}, nil, fmt.Errorf("lock subscription for alert write: %w", err)
	}
	return subscription, transaction, nil
}

type subscriptionTx struct {
	tx             pgx.Tx
	userID         int64
	subscriptionID int64
}

func (w subscriptionTx) Commit(ctx context.Context) error {
	if err := w.tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit alert transaction: %w", err)
	}
	return nil
}

func (w subscriptionTx) Rollback(ctx context.Context) error {
	return w.tx.Rollback(ctx)
}

func (w subscriptionTx) Get(ctx context.Context, alertID int64) (domain.Alert, error) {
	var alert domain.Alert
	err := w.tx.QueryRow(ctx, `
		SELECT id, type, value::text, direction, reference_price::text, is_active, is_notified
		FROM market_mind.alerts
		WHERE subscription_id = $1 AND id = $2
		FOR UPDATE
	`, w.subscriptionID, alertID).
		Scan(&alert.ID, &alert.Type, &alert.Value, &alert.Direction, &alert.ReferencePrice, &alert.IsActive, &alert.IsNotified)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Alert{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Alert{}, fmt.Errorf("read alert for update: %w", err)
	}
	return alert, nil
}

func (w subscriptionTx) Insert(ctx context.Context, alert domain.Alert) (int64, error) {
	var id int64
	err := w.tx.QueryRow(ctx, `
		INSERT INTO market_mind.alerts (subscription_id, type, value, direction, reference_price, is_active)
		VALUES ($1, $2, $3::text::numeric, $4, $5::text::numeric, $6)
		RETURNING id
	`, w.subscriptionID, alert.Type, alert.Value, alert.Direction, alert.ReferencePrice, alert.IsActive).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert alert: %w", err)
	}
	return id, nil
}

func (w subscriptionTx) Update(ctx context.Context, alert domain.Alert) error {
	tag, err := w.tx.Exec(ctx, `
		UPDATE market_mind.alerts
		SET type = $3, value = $4::text::numeric, direction = $5,
		    reference_price = $6::text::numeric, is_active = $7, is_notified = $8
		WHERE subscription_id = $1 AND id = $2
	`, w.subscriptionID, alert.ID, alert.Type, alert.Value, alert.Direction, alert.ReferencePrice, alert.IsActive, alert.IsNotified)
	if err != nil {
		return fmt.Errorf("update alert: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (w subscriptionTx) Delete(ctx context.Context, alertID int64) error {
	tag, err := w.tx.Exec(ctx, `
		DELETE FROM market_mind.alerts WHERE subscription_id = $1 AND id = $2
	`, w.subscriptionID, alertID)
	if err != nil {
		return fmt.Errorf("delete alert: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (w subscriptionTx) CountActive(ctx context.Context) (int, error) {
	var count int
	err := w.tx.QueryRow(ctx, `
		SELECT count(*)
		FROM market_mind.alerts AS a
		JOIN market_mind.subscriptions AS s ON s.id = a.subscription_id
		WHERE s.user_id = $1 AND a.is_active
	`, w.userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count user's active alerts: %w", err)
	}
	return count, nil
}
