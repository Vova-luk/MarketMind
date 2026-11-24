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

func TestIT17_AlertLimitPerSubscription(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	env := testutil.New(t, ctx)
	market := seedMarket(t, ctx, env.DB, spotMarket(binance, "IT"+rand.Text(), "90"))
	owner := env.RegisterUser(t, ctx)
	otherUser := env.RegisterUser(t, ctx)
	subscription := addSubscription(t, ctx, owner, market)
	otherSubscription := addSubscription(t, ctx, otherUser, market)
	const perSubscriptionLimit = 10
	var firstAlert alertResponse
	for i := range perSubscriptionLimit {
		alert := addRisingPriceAlert(t, ctx, owner, subscription.ID, fmt.Sprint(100+i))
		if i == 0 {
			firstAlert = alert
		}
	}

	collection := subscriptionAlertsPath(subscription.ID)
	resource := fmt.Sprintf("%s/%d", collection, firstAlert.ID)
	owner.API.RequestJSON(t, ctx, http.MethodPatch, resource, map[string]bool{"is_active": false}, http.StatusOK, nil)
	condition := map[string]any{"type": alerts.Price, "value": "200", "direction": alerts.GreaterOrEqual}
	assertAPIError(t, ctx, owner.API, http.MethodPost, collection, condition,
		http.StatusConflict, "SUBSCRIPTION_ALERT_LIMIT_REACHED")
	addRisingPriceAlert(t, ctx, otherUser, otherSubscription.ID, "200")

	owner.API.RequestJSON(t, ctx, http.MethodDelete, resource, nil, http.StatusNoContent, nil)
	addRisingPriceAlert(t, ctx, owner, subscription.ID, "200")
}
