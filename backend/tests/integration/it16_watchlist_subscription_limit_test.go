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
)

func TestIT16_WatchlistSubscriptionLimitPerUser(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	env := testutil.New(t, ctx)
	owner := env.RegisterUser(t, ctx)
	otherUser := env.RegisterUser(t, ctx)
	firstMarket := seedMarket(t, ctx, env.DB, spotMarket(binance, "IT"+rand.Text(), "90"))
	first := addSubscription(t, ctx, owner, firstMarket)
	const subscriptionLimit = 10
	for range subscriptionLimit - 1 {
		market := seedMarket(t, ctx, env.DB, spotMarket(binance, "IT"+rand.Text(), "90"))
		addSubscription(t, ctx, owner, market)
	}

	extraMarket := seedMarket(t, ctx, env.DB, spotMarket(binance, "IT"+rand.Text(), "90"))
	assertAPIError(t, ctx, owner.API, http.MethodPost, subscriptionsPath, map[string]any{
		"exchange_id": extraMarket.Exchange.id, "external_id": extraMarket.ExternalID,
	}, http.StatusConflict, "SUBSCRIPTION_LIMIT_REACHED")
	addSubscription(t, ctx, otherUser, extraMarket)

	var repeated subscriptionResponse
	owner.API.RequestJSON(t, ctx, http.MethodPost, subscriptionsPath, map[string]any{
		"exchange_id": firstMarket.Exchange.id, "external_id": firstMarket.ExternalID,
	}, http.StatusOK, &repeated)
	if repeated.ID != first.ID {
		t.Fatalf("duplicate subscription at the limit: got ID %d, want %d", repeated.ID, first.ID)
	}

	owner.API.RequestJSON(t, ctx, http.MethodDelete, fmt.Sprintf("%s/%d", subscriptionsPath, first.ID), nil, http.StatusNoContent, nil)
	addSubscription(t, ctx, owner, extraMarket)
}
