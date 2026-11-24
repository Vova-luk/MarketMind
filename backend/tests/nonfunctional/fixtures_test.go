//go:build nonfunctional

package nonfunctional

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"os"
	"slices"
	"testing"
	"time"

	"marketmind-tests/internal/testutil"
	"marketmind/api/pkg/alerts"
)

const subscriptionsPath = "/api/v1/subscriptions"

type subscription struct {
	ID           int64  `json:"subscription_id,string"`
	BaseAsset    string `json:"base_asset"`
	QuoteAsset   string `json:"quote_asset"`
	ExchangeName string `json:"exchange_name"`
}

type alert struct {
	ID             int64            `json:"alert_id,string"`
	Type           alerts.Kind      `json:"type"`
	Value          string           `json:"value"`
	Direction      alerts.Direction `json:"direction"`
	ReferencePrice string           `json:"reference_price"`
	TargetPrice    string           `json:"target_price"`
	IsActive       bool             `json:"is_active"`
}

type alertFixture struct {
	user         testutil.RegisteredUser
	subscription subscription
	alert        alert
	symbol       string
}

func createAlertFixture(t testing.TB, ctx context.Context, env testutil.Environment) alertFixture {
	t.Helper()
	baseAsset := "NF" + rand.Text()
	fixture := alertFixture{user: env.RegisterUser(t, ctx), symbol: baseAsset + "USDT"}
	fixture.user.API.RequestJSON(t, ctx, http.MethodPost, subscriptionsPath, map[string]any{
		"exchange_id": 1, "external_id": fixture.symbol,
		"base_asset": baseAsset, "quote_asset": "USDT", "price": "90",
	}, http.StatusCreated, &fixture.subscription)
	wantSubscription := subscription{ID: fixture.subscription.ID, BaseAsset: baseAsset, QuoteAsset: "USDT", ExchangeName: "Binance"}
	if fixture.subscription.ID <= 0 || fixture.subscription != wantSubscription {
		t.Fatalf("create subscription: got %+v, want %+v with a positive ID", fixture.subscription, wantSubscription)
	}
	fixture.user.API.RequestJSON(t, ctx, http.MethodPost, fixture.alertsPath(), map[string]any{
		"type": alerts.Price, "direction": alerts.GreaterOrEqual, "value": "100",
	}, http.StatusCreated, &fixture.alert)
	wantAlert := alert{ID: fixture.alert.ID, Type: alerts.Price, Direction: alerts.GreaterOrEqual,
		Value: "100", ReferencePrice: "90.00", TargetPrice: "100.00", IsActive: true}
	if fixture.alert.ID <= 0 || fixture.alert != wantAlert {
		t.Fatalf("create alert: got %+v, want %+v with a positive ID", fixture.alert, wantAlert)
	}
	return fixture
}

func (f alertFixture) alertsPath() string {
	return fmt.Sprintf("%s/%d/alerts", subscriptionsPath, f.subscription.ID)
}

func extendedRun() bool { return os.Getenv("TEST_NF_EXTENDED") == "true" }

func assertLatency(t testing.TB, scenario string, samples []time.Duration, limit time.Duration) {
	t.Helper()
	slices.Sort(samples)
	percentile := func(p int) time.Duration {
		return samples[(p*len(samples)+99)/100-1]
	}
	p95 := percentile(95)
	t.Logf("%s: n=%d p50=%s p95=%s max=%s, p95 limit=%s", scenario, len(samples), percentile(50), p95, samples[len(samples)-1], limit)
	if p95 > limit {
		t.Errorf("%s: p95 %s exceeds %s", scenario, p95, limit)
	}
}
