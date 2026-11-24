//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"slices"
	"testing"
	"time"

	"marketmind-tests/internal/testutil"
	"marketmind/api/pkg/alerts"
)

func TestIT13_PercentAlertReferencePrice(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	env := testutil.New(t, ctx)
	market := seedMarket(t, ctx, env.DB, spotMarket(binance, "IT"+rand.Text(), "100"))
	user := env.RegisterUser(t, ctx)
	subscription := addSubscription(t, ctx, user, market)
	path := subscriptionAlertsPath(subscription.ID)

	type percentAlert struct {
		alertResponse
		TargetPrice string `json:"target_price"`
	}
	condition := func(value string) map[string]any {
		return map[string]any{"type": alerts.Percent, "value": value, "direction": alerts.LessOrEqual}
	}
	var created percentAlert
	user.API.RequestJSON(t, ctx, http.MethodPost, path, condition("-10"), http.StatusCreated, &created)
	want := percentAlert{alertResponse: alertResponse{
		ID: created.ID, Type: alerts.Percent, Direction: alerts.LessOrEqual, Value: "-10", ReferencePrice: "100.00", IsActive: true,
	}, TargetPrice: "90.00"}
	if created.ID <= 0 || created != want {
		t.Fatalf("created percent alert: got %+v, want %+v with a positive ID", created, want)
	}
	assertListed := func(want percentAlert) {
		t.Helper()
		var list struct {
			Items []percentAlert `json:"items"`
		}
		user.API.RequestJSON(t, ctx, http.MethodGet, path, nil, http.StatusOK, &list)
		if !slices.Equal(list.Items, []percentAlert{want}) {
			t.Fatalf("listed percent alert: got %+v, want [%+v]", list.Items, want)
		}
	}

	applyQuote(t, ctx, testutil.NewKafka(t), env.DB, market, "120")
	assertListed(want)
	var updated percentAlert
	user.API.RequestJSON(t, ctx, http.MethodPatch, fmt.Sprintf("%s/%d", path, created.ID), condition("-20"), http.StatusOK, &updated)
	want.Value, want.ReferencePrice, want.TargetPrice = "-20", "120.00", "96.00"
	if updated != want {
		t.Fatalf("updated percent alert: got %+v, want %+v", updated, want)
	}
	assertListed(want)
}
