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
	var alertIDs [perSubscriptionLimit]int64
	for i := range perSubscriptionLimit {
		alertIDs[i] = addRisingPriceAlert(t, ctx, owner, subscription.ID, fmt.Sprint(100+i)).ID
	}

	collection := subscriptionAlertsPath(subscription.ID)
	condition := map[string]any{"type": alerts.Price, "value": "200", "direction": alerts.GreaterOrEqual}
	assertAPIError(t, ctx, owner.API, http.MethodPost, collection, condition,
		http.StatusConflict, "SUBSCRIPTION_ALERT_LIMIT_REACHED")

	for _, alertID := range alertIDs {
		resource := fmt.Sprintf("%s/%d", collection, alertID)
		owner.API.RequestJSON(t, ctx, http.MethodPatch, resource, map[string]bool{"is_active": false}, http.StatusOK, nil)
	}
	assertAPIError(t, ctx, owner.API, http.MethodPost, collection, condition,
		http.StatusConflict, "SUBSCRIPTION_ALERT_LIMIT_REACHED")

	var list struct {
		Items []alertResponse `json:"items"`
	}
	owner.API.RequestJSON(t, ctx, http.MethodGet, collection+"?page_size=100", nil, http.StatusOK, &list)
	if len(list.Items) != perSubscriptionLimit {
		t.Fatalf("stored alerts: got %d, want %d", len(list.Items), perSubscriptionLimit)
	}
	for _, alert := range list.Items {
		if alert.IsActive {
			t.Fatalf("alert %d is still active", alert.ID)
		}
	}
	addRisingPriceAlert(t, ctx, otherUser, otherSubscription.ID, "200")

	owner.API.RequestJSON(t, ctx, http.MethodDelete, fmt.Sprintf("%s/%d", collection, alertIDs[0]), nil, http.StatusNoContent, nil)
	addRisingPriceAlert(t, ctx, owner, subscription.ID, "200")
}
