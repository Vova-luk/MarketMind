//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"testing"
	"time"

	"marketmind-tests/internal/testutil"
	"marketmind/contracts/events"
)

func TestIT10_AlertIsolation(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	env := testutil.New(t, ctx)
	baseAsset := "IT" + rand.Text()
	ownerMarket := seedMarket(t, ctx, env.DB, spotMarket(binance, baseAsset, "90"))
	otherMarket := seedMarket(t, ctx, env.DB, spotMarket(okx, baseAsset, "90"))
	owner := env.RegisterUser(t, ctx)
	otherUser := env.RegisterUser(t, ctx)
	ownerSubscription := addSubscription(t, ctx, owner, ownerMarket)
	otherSubscription := addSubscription(t, ctx, otherUser, otherMarket)
	alert := addRisingPriceAlert(t, ctx, owner, ownerSubscription.ID, "100")
	ownerPath := subscriptionAlertsPath(ownerSubscription.ID)

	assertNotFound(t, ctx, otherUser.API, http.MethodGet, ownerPath, nil, "SUBSCRIPTION_NOT_FOUND")
	patch := map[string]bool{"is_active": false}
	assertNotFound(t, ctx, otherUser.API, http.MethodPatch, fmt.Sprintf("%s/%d", ownerPath, alert.ID), patch, "SUBSCRIPTION_NOT_FOUND")
	assertNotFound(t, ctx, otherUser.API, http.MethodPatch,
		fmt.Sprintf("%s/%d", subscriptionAlertsPath(otherSubscription.ID), alert.ID), patch, "ALERT_NOT_FOUND")
	var ownerAlerts struct {
		Items []alertResponse `json:"items"`
	}
	owner.API.RequestJSON(t, ctx, http.MethodGet, ownerPath, nil, http.StatusOK, &ownerAlerts)
	if !slices.Equal(ownerAlerts.Items, []alertResponse{alert}) {
		t.Fatalf("owner's alerts after rejected writes: got %+v, want [%+v]", ownerAlerts.Items, alert)
	}

	kafka := testutil.NewKafka(t)
	output := testutil.ObserveTopic[events.PriceChangedEvent](t, ctx, kafka, events.PriceChangedTopic)
	applyQuote(t, ctx, kafka, env.DB, otherMarket, "105")
	if got := userNotifications(output.ReadAvailable(t, ctx), owner.TelegramID, alert.ID); len(got) != 0 {
		t.Fatalf("quote from %s triggered the owner's %s alert: %+v", otherMarket.Exchange.name, ownerMarket.Exchange.name, got)
	}
	applyQuote(t, ctx, kafka, env.DB, ownerMarket, "105")
	got := userNotifications(output.ReadAvailable(t, ctx), owner.TelegramID, alert.ID)
	if len(got) != 1 || got[0].AlertID != alert.ID || got[0].TelegramUserID != owner.TelegramID ||
		got[0].MarketID != strconv.FormatInt(ownerMarket.ID, 10) || got[0].ExchangeName != "Binance" || got[0].CurrentPrice != "105" {
		t.Fatalf("quote from Binance market %d: got %+v, want one notification for alert %d and user %d at 105",
			ownerMarket.ID, got, alert.ID, owner.TelegramID)
	}
}

func assertNotFound(t testing.TB, ctx context.Context, api testutil.Client, method, path string, payload any, wantCode string) {
	t.Helper()
	var response struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	api.RequestJSON(t, ctx, method, path, payload, http.StatusNotFound, &response)
	if response.Error.Code != wantCode {
		t.Fatalf("%s %s: error code %q, want %q", method, path, response.Error.Code, wantCode)
	}
}
