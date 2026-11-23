//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"slices"
	"strconv"
	"testing"
	"time"

	"marketmind-tests/internal/testutil"
	"marketmind/api/pkg/alerts"
	"marketmind/contracts/events"
)

func TestIT07_AlertTrigger(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	env := testutil.New(t, ctx)
	market := seedMarket(t, ctx, env.DB, spotMarket(binance, "IT"+rand.Text(), "90"))
	user := env.RegisterUser(t, ctx)
	subscription := addSubscription(t, ctx, user, market)
	alert := addRisingPriceAlert(t, ctx, user, subscription.ID, "100")

	kafka := testutil.NewKafka(t)
	output := testutil.ObserveTopic[events.PriceChangedEvent](t, ctx, kafka, events.PriceChangedTopic)
	inputID := applyQuote(t, ctx, kafka, env.DB, market, "105")

	want := events.PriceNotification{
		NotificationID: inputID + ":" + strconv.FormatInt(alert.ID, 10),
		TelegramUserID: user.TelegramID, AlertID: alert.ID, MarketID: strconv.FormatInt(market.ID, 10),
		ExchangeName: "Binance", BaseAsset: market.BaseAsset, QuoteAsset: "USDT",
		Type: alerts.Price, Direction: alerts.GreaterOrEqual,
		ReferencePrice: "90", CurrentPrice: "105", Value: "100",
	}
	batch := output.ReadAvailable(t, ctx)
	got := userNotifications(batch, user.TelegramID, alert.ID)
	if len(got) != 1 || got[0] != want {
		t.Fatalf("notifications after crossing 100 from 90 to 105: got %+v, want [%+v]", got, want)
	}
	wantEventID := inputID + ":" + want.MarketID
	if !slices.ContainsFunc(batch, func(event events.PriceChangedEvent) bool {
		return event.EventID == wantEventID && slices.Contains(event.Items, want)
	}) {
		t.Errorf("expected event %q containing notification %q, got %+v", wantEventID, want.NotificationID, batch)
	}
}
