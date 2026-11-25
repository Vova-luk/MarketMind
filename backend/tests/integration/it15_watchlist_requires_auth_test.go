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
)

func TestIT15_WatchlistRequiresTelegramAuth(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	env := testutil.New(t, ctx)
	market := seedMarket(t, ctx, env.DB, spotMarket(binance, "IT"+rand.Text(), "90"))
	user := env.RegisterUser(t, ctx)
	subscription := addSubscription(t, ctx, user, market)
	anonymous := testutil.ClientFromEnv(t, "TEST_API_URL")

	assertAPIError(t, ctx, anonymous, http.MethodGet, subscriptionsPath, nil, http.StatusUnauthorized, "TELEGRAM_AUTH_INVALID")
	path := fmt.Sprintf("%s/%d", subscriptionsPath, subscription.ID)
	assertAPIError(t, ctx, anonymous, http.MethodDelete, path, nil, http.StatusUnauthorized, "TELEGRAM_AUTH_INVALID")

	var list watchlistResponse
	user.API.RequestJSON(t, ctx, http.MethodGet, subscriptionsPath, nil, http.StatusOK, &list)
	if !slices.Equal(list.Items, []subscriptionResponse{subscription}) {
		t.Fatalf("owner's watchlist after unauthorized deletion: got %+v, want [%+v]", list.Items, subscription)
	}
}

func assertAPIError(t testing.TB, ctx context.Context, api testutil.Client, method, path string, payload any, status int, code string) {
	t.Helper()
	var response struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	api.RequestJSON(t, ctx, method, path, payload, status, &response)
	if response.Error.Code != code {
		t.Fatalf("%s %s: error code %q, want %q", method, path, response.Error.Code, code)
	}
}
