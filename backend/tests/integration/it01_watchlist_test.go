//go:build integration || smoke

package integration

import (
	"context"
	"fmt"
	"marketmind-tests/integration/utils"
	"marketmind-tests/internal/testutil"
	"net/http"
	"slices"
	"testing"
	"time"
)

type subscriptionResponse struct {
	ID           int64  `json:"subscription_id,string"`
	BaseAsset    string `json:"base_asset"`
	QuoteAsset   string `json:"quote_asset"`
	ExchangeName string `json:"exchange_name"`
}

type watchlistResponse struct {
	Items []subscriptionResponse `json:"items"`
}

func TestIT01_WatchlistLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	env := testutil.New(t, ctx)
	marketID := utils.SeedBTCUSDTMarket(t, ctx, env.DB)
	user := env.RegisterUser(t, ctx)

	var created subscriptionResponse
	user.API.RequestJSON(t, ctx, http.MethodPost, utils.SubscriptionsPath, map[string]any{
		"exchange_id": utils.BinanceExchangeID,
		"external_id": utils.BtcusdtExternalID,
	}, http.StatusCreated, &created)
	want := subscriptionResponse{
		ID: created.ID, BaseAsset: "BTC", QuoteAsset: "USDT", ExchangeName: "Binance",
	}
	if created != want {
		t.Fatalf("created subscription: got %+v, want %+v", created, want)
	}

	var afterAdd watchlistResponse
	user.API.RequestJSON(t, ctx, http.MethodGet, utils.SubscriptionsPath, nil, http.StatusOK, &afterAdd)
	if !slices.Equal(afterAdd.Items, []subscriptionResponse{want}) {
		t.Fatalf("watchlist after add: got %+v, want [%+v]", afterAdd.Items, want)
	}
	utils.AssertStoredWatchlist(t, ctx, env.DB, user.TelegramID, created.ID,
		[]utils.StoredSubscription{{ID: created.ID, MarketID: marketID, TelegramID: user.TelegramID}})

	user.API.RequestJSON(t, ctx, http.MethodDelete, fmt.Sprintf("%s/%d", utils.SubscriptionsPath, created.ID),
		nil, http.StatusNoContent, nil)

	var afterDelete watchlistResponse
	user.API.RequestJSON(t, ctx, http.MethodGet, utils.SubscriptionsPath, nil, http.StatusOK, &afterDelete)
	if afterDelete.Items == nil || len(afterDelete.Items) != 0 {
		t.Fatalf("watchlist after delete: got %#v, want an empty items array", afterDelete.Items)
	}
	utils.AssertStoredWatchlist(t, ctx, env.DB, user.TelegramID, created.ID, nil)
}
