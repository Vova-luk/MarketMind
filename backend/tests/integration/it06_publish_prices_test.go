//go:build integration

package integration

import (
	"context"
	"strconv"
	"testing"
	"time"

	"marketmind-tests/internal/testutil"
	"marketmind/contracts/events"
)

func TestIT06_PublishPrices(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	env := testutil.New(t, ctx)
	kafka := testutil.NewKafka(t)
	quotes := testutil.ObserveTopic[events.PricesCheckedEvent](t, ctx, kafka, events.PricesCheckedTopic)

	// Keep this shared fixture until the integration volume is removed. A poll
	// already in flight can publish again after the test user's subscription is deleted.
	market, _ := ensureMarket(t, ctx, env.DB, spotMarket(bybit, "IT06", "90"))
	user := env.RegisterUser(t, ctx)
	addSubscription(t, ctx, user, market)
	marketID := strconv.FormatInt(market.ID, 10)

	testutil.Eventually(t, ctx, "IT06USDT quote in Price-Checked-Topic", func() bool {
		for _, event := range quotes.ReadAvailable(t, ctx) {
			for _, item := range event.Items {
				if item.MarketID != marketID && item.ExternalID != "IT06USDT" {
					continue
				}
				if event.EventID == "" || item.ReceivedAt.IsZero() || item.ExchangeName != "bybit" ||
					item.MarketID != marketID || item.ExternalID != "IT06USDT" || item.Price != "100" {
					t.Fatalf("quote for market %s: got %+v in event %q, want Bybit IT06USDT at 100 with event ID and timestamp",
						marketID, item, event.EventID)
				}
				return true
			}
		}
		return false
	})
}
