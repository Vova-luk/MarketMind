//go:build nonfunctional

package nonfunctional

import (
	"context"
	"net/http"
	"testing"
	"time"

	"marketmind-tests/internal/testutil"
)

func TestNF02_ExchangeOutageAndRecovery(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	const searchPath = "/api/v1/markets?exchange_id=2&q=IT06USDT"
	bybit := testutil.ClientFromEnv(t, "TEST_BYBIT_STUB_URL")
	setFault := func(ctx context.Context, mode string) {
		bybit.RequestJSON(t, ctx, http.MethodPut, "/ticker-fault", map[string]string{"mode": mode}, http.StatusNoContent, nil)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		setFault(ctx, "healthy")
	})
	user := testutil.New(t, ctx).RegisterUser(t, ctx)
	// Eliminate a cached successful catalog response from an earlier run.
	stack := testutil.OpenTestStack(t, ctx)
	stack.Restart(t, ctx, "exchange-data")
	apiStarted := stack.StartedAt(t, ctx, "api")
	exchangeStarted := stack.StartedAt(t, ctx, "exchange-data")
	for _, mode := range []string{"timeout", "unavailable"} {
		setFault(ctx, mode)
		var response struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
		}
		started := time.Now()
		user.API.RequestJSON(t, ctx, http.MethodGet, searchPath, nil, http.StatusServiceUnavailable, &response)
		if response.Error.Code != "MARKET_CATALOG_UNAVAILABLE" {
			t.Fatalf("Bybit %s: got error code %q, want MARKET_CATALOG_UNAVAILABLE", mode, response.Error.Code)
		}
		if mode == "timeout" && time.Since(started) < 500*time.Millisecond {
			t.Fatal("catalog returned before the Bybit timeout occurred")
		}
		user.API.RequestJSON(t, ctx, http.MethodGet, "/health", nil, http.StatusOK, nil)
	}

	setFault(ctx, "healthy")
	var recovered struct {
		Items []struct {
			ExternalID string `json:"external_id"`
			PriceExact string `json:"price_exact"`
		} `json:"items"`
	}
	user.API.RequestJSON(t, ctx, http.MethodGet, searchPath, nil, http.StatusOK, &recovered)
	if len(recovered.Items) != 1 || recovered.Items[0].ExternalID != "IT06USDT" || recovered.Items[0].PriceExact != "100" {
		t.Fatalf("recovered Bybit quote: got %+v, want IT06USDT at 100", recovered.Items)
	}
	for service, started := range map[string]string{"api": apiStarted, "exchange-data": exchangeStarted} {
		if got := stack.StartedAt(t, ctx, service); got != started {
			t.Errorf("%s restarted during the outage: started at %s, previously %s", service, got, started)
		}
	}
}
