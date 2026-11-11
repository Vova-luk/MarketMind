package subscriptions

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	domain "marketmind/api/internal/domain/subscriptions"
)

func (r *Repository) BeginAdd(ctx context.Context, telegramUserID int64) (domain.AddTransaction, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return nil, fmt.Errorf("begin subscription transaction: %w", err)
	}
	add := addTx{tx: tx}
	// Serialize additions for this user, including the limit check.
	err = tx.QueryRow(ctx, `
		SELECT id FROM market_mind.users
		WHERE telegram_user_id = $1
		FOR NO KEY UPDATE
	`, telegramUserID).Scan(&add.userID)
	if err != nil {
		_ = tx.Rollback(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotRegistered
		}
		return nil, fmt.Errorf("lock user for subscription add: %w", err)
	}
	return add, nil
}

type addTx struct {
	tx     pgx.Tx
	userID int64
}

func (a addTx) GetExchange(ctx context.Context, exchangeID int32) (domain.Exchange, error) {
	var exchange domain.Exchange
	err := a.tx.QueryRow(ctx, `
		SELECT name, is_active FROM market_mind.exchanges WHERE id = $1 FOR SHARE
	`, exchangeID).Scan(&exchange.Name, &exchange.Active)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Exchange{}, domain.ErrExchangeNotFound
	}
	if err != nil {
		return domain.Exchange{}, fmt.Errorf("read exchange: %w", err)
	}
	return exchange, nil
}

func (a addTx) FindMarket(ctx context.Context, key domain.MarketKey, exchange domain.Exchange) (domain.Market, error) {
	market := domain.Market{Subscription: domain.Subscription{ExchangeName: exchange.Name}}
	err := a.tx.QueryRow(ctx, `
		SELECT m.id, m.base_asset, m.quote_asset,
		       CASE WHEN $4::boolean THEN m.price::text END, COALESCE(s.id, 0)
		FROM market_mind.markets AS m
		LEFT JOIN market_mind.subscriptions AS s ON s.market_id = m.id AND s.user_id = $3
		WHERE m.exchange_id = $1 AND m.external_id = $2
		FOR KEY SHARE OF m
	`, key.ExchangeID, key.ExternalID, a.userID, exchange.Active).
		Scan(&market.MarketID, &market.BaseAsset, &market.QuoteAsset, &market.Price, &market.Subscription.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Market{}, nil
	}
	if err != nil {
		return domain.Market{}, fmt.Errorf("find market for subscription: %w", err)
	}
	return market, nil
}

func (a addTx) CountSubscriptions(ctx context.Context) (int, error) {
	var count int
	err := a.tx.QueryRow(ctx, `
		SELECT count(*) FROM market_mind.subscriptions WHERE user_id = $1
	`, a.userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count user's subscriptions: %w", err)
	}
	return count, nil
}

func (a addTx) InsertMarket(ctx context.Context, key domain.MarketKey, exchange domain.Exchange, seed domain.MarketSeed) (domain.Market, error) {
	market := domain.Market{Subscription: domain.Subscription{ExchangeName: exchange.Name}}
	err := a.tx.QueryRow(ctx, `
		INSERT INTO market_mind.markets (exchange_id, external_id, base_asset, quote_asset, price)
		VALUES ($1, $2, $3, $4, $5::text::numeric)
		ON CONFLICT DO NOTHING
		RETURNING id, base_asset, quote_asset, price::text
	`, key.ExchangeID, key.ExternalID, seed.BaseAsset, seed.QuoteAsset, seed.Price).
		Scan(&market.MarketID, &market.BaseAsset, &market.QuoteAsset, &market.Price)
	if errors.Is(err, pgx.ErrNoRows) {
		// Another transaction may have inserted this market after our first read.
		market, err = a.FindMarket(ctx, key, exchange)
		if err != nil {
			return domain.Market{}, err
		}
		if market.MarketID == 0 {
			return domain.Market{}, domain.ErrMarketConflict
		}
		return market, nil
	}
	if err != nil {
		return domain.Market{}, fmt.Errorf("insert market: %w", err)
	}
	return market, nil
}

func (a addTx) InsertSubscription(ctx context.Context, marketID int64) (int64, error) {
	var id int64
	err := a.tx.QueryRow(ctx, `
		INSERT INTO market_mind.subscriptions (user_id, market_id)
		VALUES ($1, $2)
		RETURNING id
	`, a.userID, marketID).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert subscription: %w", err)
	}
	return id, nil
}

func (a addTx) Commit(ctx context.Context) error {
	if err := a.tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit subscription transaction: %w", err)
	}
	return nil
}

func (a addTx) Rollback(ctx context.Context) error {
	return a.tx.Rollback(ctx)
}
