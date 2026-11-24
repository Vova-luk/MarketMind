//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"marketmind-tests/internal/testutil"
	"marketmind/contracts/events"
)

// applyQuote waits for Tracking to persist this exact input. Tracking publishes
// notifications before saving the price, so the Kafka output can then be read to its end.
func applyQuote(t testing.TB, ctx context.Context, kafka *testutil.Kafka, db *pgx.Conn, market marketFixture, price string) string {
	t.Helper()
	_, previous := testutil.MarketPrice(t, ctx, db, market.ID)
	receivedAt := time.Now().UTC().Truncate(time.Microsecond)
	if !receivedAt.After(previous) {
		receivedAt = previous.Add(time.Microsecond)
	}
	marketID := strconv.FormatInt(market.ID, 10)
	eventID := rand.Text()
	kafka.PublishJSON(t, ctx, events.PricesCheckedTopic, marketID, events.PricesCheckedEvent{
		EventID: eventID,
		Items: []events.CheckedPrice{{
			ExchangeName: strings.ToLower(market.Exchange.name), MarketID: marketID,
			ExternalID: market.ExternalID, Price: price, ReceivedAt: receivedAt,
		}},
	})
	testutil.Eventually(t, ctx, fmt.Sprintf("market %d to store quote %s from event %s", market.ID, price, eventID), func() bool {
		stored, updatedAt := testutil.MarketPrice(t, ctx, db, market.ID)
		return stored == price && updatedAt.Equal(receivedAt)
	})
	return eventID
}

func userNotifications(batch []events.PriceChangedEvent, userID, alertID int64) []events.PriceNotification {
	var notifications []events.PriceNotification
	for _, event := range batch {
		for _, item := range event.Items {
			if item.TelegramUserID == userID || item.AlertID == alertID {
				notifications = append(notifications, item)
			}
		}
	}
	return notifications
}
