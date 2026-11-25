//go:build e2e

package endtoend

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"marketmind-tests/internal/testutil"
	"marketmind/api/pkg/alerts"
	"marketmind/contracts/events"
)

func TestE2E03_PercentDropNotification(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	const initialPrice, reachedPrice = "200", "190"
	baseAsset := "E2E" + rand.Text()
	symbol := baseAsset + "USDT"
	bybit := testutil.ClientFromEnv(t, "TEST_BYBIT_STUB_URL")
	telegram := testutil.ClientFromEnv(t, "TEST_TELEGRAM_STUB_URL")
	pricePath := "/prices/" + symbol
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		bybit.RequestJSON(t, ctx, http.MethodDelete, pricePath, nil, http.StatusNoContent, nil)
	})
	setPrice := func(price string) {
		bybit.RequestJSON(t, ctx, http.MethodPut, pricePath, map[string]string{"price": price}, http.StatusNoContent, nil)
	}
	setPrice(initialPrice)

	env := testutil.New(t, ctx)
	user := env.RegisterUser(t, ctx)
	kafka := testutil.NewKafka(t)
	output := testutil.ObserveTopic[events.PriceChangedEvent](t, ctx, kafka, events.PriceChangedTopic)
	var subscription struct {
		ID int64 `json:"subscription_id,string"`
	}
	user.API.RequestJSON(t, ctx, http.MethodPost, "/api/v1/subscriptions", map[string]any{
		"exchange_id": 2, "external_id": symbol,
		"base_asset": baseAsset, "quote_asset": "USDT", "price": initialPrice,
	}, http.StatusCreated, &subscription)
	// Keep the market until stack teardown so Tracking can process in-flight quotes.
	var marketID int64
	if err := env.DB.QueryRow(ctx, `SELECT market_id FROM market_mind.subscriptions WHERE id = $1`, subscription.ID).Scan(&marketID); err != nil {
		t.Fatalf("look up market for subscription %d: %v", subscription.ID, err)
	}
	marketKey := strconv.FormatInt(marketID, 10)
	var alert struct {
		ID          int64  `json:"alert_id,string"`
		TargetPrice string `json:"target_price"`
		IsActive    bool   `json:"is_active"`
	}
	alertsPath := fmt.Sprintf("/api/v1/subscriptions/%d/alerts", subscription.ID)
	user.API.RequestJSON(t, ctx, http.MethodPost, alertsPath, map[string]any{
		"type": alerts.Percent, "value": "-5", "direction": alerts.LessOrEqual,
	}, http.StatusCreated, &alert)
	if alert.ID <= 0 || !alert.IsActive || alert.TargetPrice != "190.00" {
		t.Fatalf("percentage alert: got %+v, want active alert with target 190.00", alert)
	}

	_, lastUpdate := testutil.MarketPrice(t, ctx, env.DB, marketID)
	waitForQuote := func(price string) {
		testutil.Eventually(t, ctx, "Tracking to store a new quote at "+price+" for "+symbol, func() bool {
			stored, updatedAt := testutil.MarketPrice(t, ctx, env.DB, marketID)
			if stored != price || !updatedAt.After(lastUpdate) {
				return false
			}
			lastUpdate = updatedAt
			return true
		})
	}
	readNotifications := func() []events.PriceNotification {
		var notifications []events.PriceNotification
		for _, event := range output.ReadAvailable(t, ctx) {
			for _, item := range event.Items {
				if item.MarketID == marketKey || item.AlertID == alert.ID || item.TelegramUserID == user.TelegramID {
					notifications = append(notifications, item)
				}
			}
		}
		return notifications
	}
	setPrice("191")
	waitForQuote("191")
	if got := readNotifications(); len(got) != 0 {
		t.Fatalf("notifications above the target: got %+v, want none", got)
	}
	if messages := testutil.TelegramMessages(t, ctx, telegram, user.TelegramID); len(messages) != 0 {
		t.Fatalf("Telegram messages above the target: got %+v, want none", messages)
	}

	setPrice(reachedPrice)
	waitForQuote(reachedPrice)
	waitForQuote(reachedPrice) // The next poll at the same price must not trigger a second event.
	notifications := readNotifications()
	if len(notifications) != 1 {
		t.Fatalf("notifications at the target: got %+v, want exactly one", notifications)
	}
	want := events.PriceNotification{
		NotificationID: notifications[0].NotificationID, TelegramUserID: user.TelegramID, AlertID: alert.ID,
		MarketID: marketKey, ExchangeName: "Bybit", BaseAsset: baseAsset, QuoteAsset: "USDT",
		Type: alerts.Percent, Direction: alerts.LessOrEqual, Value: "-5", ReferencePrice: initialPrice, CurrentPrice: reachedPrice,
	}
	if want.NotificationID == "" || notifications[0] != want {
		t.Fatalf("notification at the target: got %+v, want %+v with an ID", notifications[0], want)
	}
	var messages []testutil.TelegramMessage
	testutil.Eventually(t, ctx, "Telegram notification for the percentage drop", func() bool {
		messages = testutil.TelegramMessages(t, ctx, telegram, user.TelegramID)
		return len(messages) > 0
	})
	if len(messages) != 1 || messages[0].ChatID != user.TelegramID || messages[0].ParseMode != "HTML" {
		t.Fatalf("Telegram delivery: got %+v, want one HTML message for chat %d", messages, user.TelegramID)
	}
	for _, fragment := range []string{baseAsset + "/USDT (Bybit)", "190.00 USDT", "≤ -5%"} {
		if !strings.Contains(messages[0].Text, fragment) {
			t.Errorf("Telegram message %q does not contain %q", messages[0].Text, fragment)
		}
	}
}
