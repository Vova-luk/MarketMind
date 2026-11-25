//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"strconv"
	"testing"
	"time"

	"marketmind-tests/internal/testutil"
	"marketmind/contracts/events"
)

func TestIT08_AlertCrossing(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	env := testutil.New(t, ctx)
	market := seedMarket(t, ctx, env.DB, spotMarket(binance, "IT"+rand.Text(), "90"))
	user := env.RegisterUser(t, ctx)
	subscription := addSubscription(t, ctx, user, market)
	alert := addRisingPriceAlert(t, ctx, user, subscription.ID, "100")

	kafka := testutil.NewKafka(t)
	output := testutil.ObserveTopic[events.PriceChangedEvent](t, ctx, kafka, events.PriceChangedTopic)
	steps := []struct {
		price string
		want  int
	}{
		{price: "90", want: 0},
		{price: "95", want: 0},
		{price: "105", want: 1},
		{price: "110", want: 0},
		{price: "90", want: 0},
		{price: "105", want: 1},
	}
	for i, step := range steps {
		inputID := applyQuote(t, ctx, kafka, env.DB, market, step.price)
		got := userNotifications(output.ReadAvailable(t, ctx), user.TelegramID, alert.ID)
		if len(got) != step.want {
			t.Fatalf("step %d at price %s: got %d notifications %+v, want %d", i+1, step.price, len(got), got, step.want)
		}
		for _, item := range got {
			if item.NotificationID != inputID+":"+strconv.FormatInt(alert.ID, 10) || item.CurrentPrice != step.price ||
				item.AlertID != alert.ID || item.TelegramUserID != user.TelegramID {
				t.Fatalf("step %d at price %s: unexpected notification %+v", i+1, step.price, item)
			}
		}
	}
}
