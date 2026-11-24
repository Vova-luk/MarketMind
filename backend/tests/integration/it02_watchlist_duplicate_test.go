//go:build integration || smoke

package integration

import (
	"context"
	"marketmind-tests/integration/utils"
	"marketmind-tests/internal/testutil"
	"net/http"
	"testing"
	"time"
)

func Test02_Watchlist_Duplicates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	if created.ID <= 0 || created != want {
		t.Fatalf("created subscription: got %+v, want a positive ID and %+v", created, want)
	}

	var repeated subscriptionResponse
	user.API.RequestJSON(t, ctx, http.MethodPost, utils.SubscriptionsPath, map[string]any{
		"exchange_id": utils.BinanceExchangeID,
		"external_id": utils.BtcusdtExternalID,
	}, http.StatusOK, &repeated)

	if repeated.ID != created.ID {
		t.Fatalf("duplicate subscription: got ID %d, want %d", repeated.ID, created.ID)
	}

	var list watchlistResponse
	user.API.RequestJSON(t, ctx, http.MethodGet, utils.SubscriptionsPath, nil, http.StatusOK, &list)

	if len(list.Items) != 1 {
		t.Fatalf("watchlist: got %d subscriptions, want 1", len(list.Items))
	}

	if list.Items[0].ID != created.ID {
		t.Fatalf("subscription ID: got %d, want %d", list.Items[0].ID, created.ID)
	}

	utils.AssertStoredWatchlist(t, ctx, env.DB, user.TelegramID, created.ID,
		[]utils.StoredSubscription{{ID: created.ID, MarketID: marketID, TelegramID: user.TelegramID}})
}
