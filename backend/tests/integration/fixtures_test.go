//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"marketmind-tests/internal/testutil"
	"marketmind/api/pkg/alerts"
)

const subscriptionsPath = "/api/v1/subscriptions"

type exchangeFixture struct {
	id   int
	name string
}

var (
	binance = exchangeFixture{id: 1, name: "Binance"}
	bybit   = exchangeFixture{id: 2, name: "Bybit"}
	okx     = exchangeFixture{id: 3, name: "OKX"}
)

type marketFixture struct {
	ID         int64
	Exchange   exchangeFixture
	ExternalID string
	BaseAsset  string
	QuoteAsset string
	Price      string
}

func spotMarket(exchange exchangeFixture, baseAsset, price string) marketFixture {
	return marketFixture{
		Exchange: exchange, ExternalID: baseAsset + "USDT", BaseAsset: baseAsset,
		QuoteAsset: "USDT", Price: price,
	}
}

func ensureMarket(t testing.TB, ctx context.Context, db *pgx.Conn, market marketFixture) (marketFixture, bool) {
	t.Helper()
	err := db.QueryRow(ctx, `
		INSERT INTO market_mind.markets (exchange_id, external_id, base_asset, quote_asset, price)
		VALUES ($1, $2, $3, $4, $5::text::numeric)
		ON CONFLICT (exchange_id, external_id) DO NOTHING
		RETURNING id
	`, market.Exchange.id, market.ExternalID, market.BaseAsset, market.QuoteAsset, market.Price).Scan(&market.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = db.QueryRow(ctx, `
			SELECT id FROM market_mind.markets WHERE exchange_id = $1 AND external_id = $2
		`, market.Exchange.id, market.ExternalID).Scan(&market.ID)
		if err != nil {
			t.Fatalf("look up %s %s fixture: %v", market.Exchange.name, market.ExternalID, err)
		}
		return market, false
	}
	if err != nil {
		t.Fatalf("insert %s %s fixture: %v", market.Exchange.name, market.ExternalID, err)
	}
	return market, true
}

func seedMarket(t testing.TB, ctx context.Context, db *pgx.Conn, market marketFixture) marketFixture {
	t.Helper()
	market, inserted := ensureMarket(t, ctx, db, market)
	if inserted {
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if _, err := db.Exec(ctx, "DELETE FROM market_mind.markets WHERE id = $1", market.ID); err != nil {
				t.Errorf("delete %s %s fixture: %v", market.Exchange.name, market.ExternalID, err)
			}
		})
	}
	return market
}

type subscriptionResponse struct {
	ID           int64  `json:"subscription_id,string"`
	BaseAsset    string `json:"base_asset"`
	QuoteAsset   string `json:"quote_asset"`
	ExchangeName string `json:"exchange_name"`
}

func addSubscription(t testing.TB, ctx context.Context, user testutil.RegisteredUser, market marketFixture) subscriptionResponse {
	t.Helper()
	var subscription subscriptionResponse
	user.API.RequestJSON(t, ctx, http.MethodPost, subscriptionsPath, map[string]any{
		"exchange_id": market.Exchange.id, "external_id": market.ExternalID,
	}, http.StatusCreated, &subscription)
	if subscription.ID <= 0 {
		t.Fatalf("create subscription for %s %s: invalid ID %d", market.Exchange.name, market.ExternalID, subscription.ID)
	}
	return subscription
}

type alertResponse struct {
	ID             int64            `json:"alert_id,string"`
	Type           alerts.Kind      `json:"type"`
	Value          string           `json:"value"`
	Direction      alerts.Direction `json:"direction"`
	ReferencePrice string           `json:"reference_price"`
	IsActive       bool             `json:"is_active"`
}

func subscriptionAlertsPath(subscriptionID int64) string {
	return fmt.Sprintf("%s/%d/alerts", subscriptionsPath, subscriptionID)
}

func addRisingPriceAlert(t testing.TB, ctx context.Context, user testutil.RegisteredUser, subscriptionID int64, target string) alertResponse {
	t.Helper()
	var alert alertResponse
	user.API.RequestJSON(t, ctx, http.MethodPost, subscriptionAlertsPath(subscriptionID), map[string]any{
		"type": alerts.Price, "value": target, "direction": alerts.GreaterOrEqual,
	}, http.StatusCreated, &alert)
	if alert.ID <= 0 || !alert.IsActive {
		t.Fatalf("create active price alert for subscription %d: got %+v", subscriptionID, alert)
	}
	return alert
}
