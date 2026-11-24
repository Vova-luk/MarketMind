//go:build integration || smoke

package integration

import (
	"context"
	"net/http"
	"testing"
	"time"

	"marketmind-tests/internal/testutil"
)

func TestIT04_AlertValidation(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	env := testutil.New(t, ctx)
	market := seedMarket(t, ctx, env.DB, spotMarket(binance, "BTC", "65000.50"))

	for _, tc := range []struct {
		name   string
		value  string
		status int
	}{
		{name: "zero", value: "0", status: http.StatusBadRequest},
		{name: "negative", value: "-1", status: http.StatusBadRequest},
		{name: "upper_boundary", value: "500000", status: http.StatusCreated},
		{name: "above_upper_boundary", value: "500000.01", status: http.StatusBadRequest},
		{name: "not_a_number", value: "abc", status: http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user := env.RegisterUser(t, ctx)
			subscription := addSubscription(t, ctx, user, market)
			payload := map[string]string{"type": "price", "value": tc.value, "direction": "gte"}

			if tc.status == http.StatusBadRequest {
				var response struct {
					Error struct {
						Code string `json:"code"`
					} `json:"error"`
				}
				user.API.RequestJSON(t, ctx, http.MethodPost, subscriptionAlertsPath(subscription.ID),
					payload, tc.status, &response)
				if response.Error.Code != "VALIDATION_ERROR" {
					t.Errorf("invalid price %q: error code = %q, want VALIDATION_ERROR", tc.value, response.Error.Code)
				}

				var count int
				if err := env.DB.QueryRow(ctx, `
					SELECT count(*) FROM market_mind.alerts WHERE subscription_id = $1
				`, subscription.ID).Scan(&count); err != nil {
					t.Fatalf("count alerts after rejected price %q: %v", tc.value, err)
				}
				if count != 0 {
					t.Fatalf("rejected price %q: stored %d alerts, want 0", tc.value, count)
				}
				return
			}

			var created alertResponse
			user.API.RequestJSON(t, ctx, http.MethodPost, subscriptionAlertsPath(subscription.ID),
				payload, tc.status, &created)
			if created.ID <= 0 || created.Type != "price" || created.Value != tc.value ||
				created.Direction != "gte" || !created.IsActive {
				t.Fatalf("accepted boundary price: unexpected alert %+v", created)
			}

			var id int64
			var matches bool
			if err := env.DB.QueryRow(ctx, `
				SELECT id, type = 'price' AND value = $2::text::numeric AND direction = 'gte' AND is_active
				FROM market_mind.alerts WHERE subscription_id = $1
			`, subscription.ID, tc.value).Scan(&id, &matches); err != nil {
				t.Fatalf("read accepted boundary price from PostgreSQL: %v", err)
			}
			if id != created.ID || !matches {
				t.Fatalf("stored boundary alert: id=%d, matches=%t; want id=%d and matching condition", id, matches, created.ID)
			}
		})
	}
}
