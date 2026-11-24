//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"testing"
	"time"

	"marketmind-tests/internal/testutil"
	"marketmind/api/pkg/alerts"
)

func TestIT14_ActiveAlertLimitPerUser(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	env := testutil.New(t, ctx)
	first := seedMarket(t, ctx, env.DB, spotMarket(binance, "IT"+rand.Text(), "90"))
	second := seedMarket(t, ctx, env.DB, spotMarket(binance, "IT"+rand.Text(), "90"))
	owner := env.RegisterUser(t, ctx)
	otherUser := env.RegisterUser(t, ctx)
	firstSubscription := addSubscription(t, ctx, owner, first)
	secondSubscription := addSubscription(t, ctx, owner, second)
	otherSubscription := addSubscription(t, ctx, otherUser, second)

	const activeLimit = 50 // Configured in the API configuration used by the test stack.
	var firstAlertID int64
	for i := range activeLimit {
		alert := addRisingPriceAlert(t, ctx, owner, firstSubscription.ID, fmt.Sprint(100+i))
		if i == 0 {
			firstAlertID = alert.ID
		}
	}
	addRisingPriceAlert(t, ctx, otherUser, otherSubscription.ID, "200")
	condition := map[string]any{"type": alerts.Price, "value": "200", "direction": alerts.GreaterOrEqual}
	assertAPIError(t, ctx, owner.API, http.MethodPost, subscriptionAlertsPath(secondSubscription.ID), condition,
		http.StatusConflict, "ACTIVE_ALERT_LIMIT_REACHED")

	alertPath := fmt.Sprintf("%s/%d", subscriptionAlertsPath(firstSubscription.ID), firstAlertID)
	var disabled alertResponse
	owner.API.RequestJSON(t, ctx, http.MethodPatch, alertPath, map[string]bool{"is_active": false}, http.StatusOK, &disabled)
	if disabled.ID != firstAlertID || disabled.IsActive {
		t.Fatalf("disabled alert: got %+v, want inactive alert %d", disabled, firstAlertID)
	}
	addRisingPriceAlert(t, ctx, owner, secondSubscription.ID, "200")
	assertAPIError(t, ctx, owner.API, http.MethodPatch, alertPath, map[string]bool{"is_active": true},
		http.StatusConflict, "ACTIVE_ALERT_LIMIT_REACHED")

	var active, total int
	if err := env.DB.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE a.is_active), count(*)
		FROM market_mind.alerts AS a
		JOIN market_mind.subscriptions AS s ON s.id = a.subscription_id
		JOIN market_mind.users AS u ON u.id = s.user_id
		WHERE u.telegram_user_id = $1
	`, owner.TelegramID).Scan(&active, &total); err != nil {
		t.Fatalf("count stored alerts: %v", err)
	}
	if active != activeLimit || total != activeLimit+1 {
		t.Fatalf("stored alerts: got %d active of %d, want %d active of %d", active, total, activeLimit, activeLimit+1)
	}
}
