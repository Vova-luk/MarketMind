//go:build nonfunctional

package nonfunctional

import (
	"context"
	"crypto/rand"
	"fmt"
	"strconv"
	"testing"
	"time"

	"marketmind-tests/internal/testutil"
	"marketmind/contracts/events"
)

func TestNF04_KafkaLatencyAndReplay(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()

	stack := testutil.OpenTestStack(t, ctx)
	env := testutil.New(t, ctx)
	fixture := createAlertFixture(t, ctx, env)
	var marketID int64
	if err := env.DB.QueryRow(ctx, `
		SELECT market_id FROM market_mind.subscriptions WHERE id = $1
	`, fixture.subscription.ID).Scan(&marketID); err != nil {
		t.Fatalf("look up market for subscription %d: %v", fixture.subscription.ID, err)
	}
	marketKey := strconv.FormatInt(marketID, 10)
	_, lastTimestamp := testutil.MarketPrice(t, ctx, env.DB, marketID)
	nextEvent := func(price string) events.PricesCheckedEvent {
		receivedAt := time.Now().UTC().Truncate(time.Microsecond)
		if !receivedAt.After(lastTimestamp) {
			receivedAt = lastTimestamp.Add(time.Microsecond)
		}
		lastTimestamp = receivedAt
		return events.PricesCheckedEvent{EventID: rand.Text(), Items: []events.CheckedPrice{{
			ExchangeName: "binance", MarketID: marketKey, ExternalID: fixture.symbol,
			Price: price, ReceivedAt: receivedAt,
		}}}
	}
	kafka := testutil.NewKafka(t)
	output := testutil.ObserveTopic[events.PriceChangedEvent](t, ctx, kafka, events.PriceChangedTopic)
	publish := func(event events.PricesCheckedEvent) {
		kafka.PublishJSON(t, ctx, events.PricesCheckedTopic, marketKey, event)
	}
	waitApplied := func(event events.PricesCheckedEvent) {
		quote := event.Items[0]
		testutil.Eventually(t, ctx, fmt.Sprintf("Tracking to apply event %s at %s", event.EventID, quote.Price), func() bool {
			price, updatedAt := testutil.MarketPrice(t, ctx, env.DB, marketID)
			return price == quote.Price && updatedAt.Equal(quote.ReceivedAt)
		})
	}
	warmup := nextEvent("90")
	publish(warmup)
	waitApplied(warmup)
	sampleCount := 20
	if extendedRun() {
		sampleCount = 60
	}
	samples := make([]time.Duration, 0, sampleCount)
	for range sampleCount {
		event := nextEvent("90")
		started := time.Now()
		publish(event)
		waitApplied(event)
		samples = append(samples, time.Since(started))
	}
	assertLatency(t, "Kafka publication to Tracking persistence (100 ms polling)", samples, 500*time.Millisecond)

	crossing := nextEvent("105")
	publish(crossing)
	waitApplied(crossing)
	telegram := testutil.ClientFromEnv(t, "TEST_TELEGRAM_STUB_URL")
	testutil.Eventually(t, ctx, "first Telegram notification", func() bool {
		return len(testutil.TelegramMessages(t, ctx, telegram, fixture.user.TelegramID)) > 0
	})

	needsRecovery := true
	t.Cleanup(func() {
		if needsRecovery {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			stack.StartAndWait(t, ctx, "tracking")
		}
	})
	stack.StopImmediately(t, ctx, "tracking")
	publish(crossing)
	barrier := nextEvent("110")
	publish(barrier)
	stack.StartAndWait(t, ctx, "tracking")
	needsRecovery = false
	waitApplied(barrier)

	var notifications []events.PriceNotification
	for _, event := range output.ReadAvailable(t, ctx) {
		for _, item := range event.Items {
			if item.MarketID == marketKey || item.AlertID == fixture.alert.ID || item.TelegramUserID == fixture.user.TelegramID {
				notifications = append(notifications, item)
			}
		}
	}
	wantNotificationID := fmt.Sprintf("%s:%d", crossing.EventID, fixture.alert.ID)
	if len(notifications) != 1 || notifications[0].NotificationID != wantNotificationID {
		t.Errorf("Kafka notifications after replay: got %+v, want one notification %q", notifications, wantNotificationID)
	}
	if messages := testutil.TelegramMessages(t, ctx, telegram, fixture.user.TelegramID); len(messages) != 1 {
		t.Errorf("Telegram messages after replay: got %+v, want exactly one", messages)
	}
}
