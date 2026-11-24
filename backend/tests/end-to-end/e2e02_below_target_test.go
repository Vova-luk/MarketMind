//go:build e2e

package endtoend

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"marketmind-tests/internal/testutil"
	"marketmind/api/pkg/alerts"
	"marketmind/contracts/events"
)

func TestE2E02_PriceStaysBelowTarget(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	const (
		bybitExchangeID = 2
		initialPrice    = "75"
		finalPrice      = "95"
		target          = "100"
	)
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

	// User cleanup removes the subscription. Retain the market until stack teardown
	// so Tracking can still apply a quote already published by the scheduler.
	var subscription struct {
		ID int64 `json:"subscription_id,string"`
	}
	user.API.RequestJSON(t, ctx, http.MethodPost, "/api/v1/subscriptions", map[string]any{
		"exchange_id": bybitExchangeID, "external_id": symbol,
		"base_asset": baseAsset, "quote_asset": "USDT", "price": initialPrice,
	}, http.StatusCreated, &subscription)
	var alert struct {
		ID       int64 `json:"alert_id,string"`
		IsActive bool  `json:"is_active"`
	}
	user.API.RequestJSON(t, ctx, http.MethodPost, fmt.Sprintf("/api/v1/subscriptions/%d/alerts", subscription.ID), map[string]any{
		"type": alerts.Price, "value": target, "direction": alerts.GreaterOrEqual,
	}, http.StatusCreated, &alert)
	if alert.ID <= 0 || !alert.IsActive {
		t.Fatalf("created alert: got %+v, want an active alert with a positive ID", alert)
	}

	var marketID int64
	if err := env.DB.QueryRow(ctx, `
		SELECT market_id FROM market_mind.subscriptions WHERE id = $1
	`, subscription.ID).Scan(&marketID); err != nil {
		t.Fatalf("look up market for subscription %d: %v", subscription.ID, err)
	}
	_, lastUpdate := testutil.MarketPrice(t, ctx, env.DB, marketID)
	waitForQuote := func(want string) {
		testutil.Eventually(t, ctx, "Tracking to store a new quote at "+want+" for "+symbol, func() bool {
			price, updatedAt := testutil.MarketPrice(t, ctx, env.DB, marketID)
			if price != want || !updatedAt.After(lastUpdate) {
				return false
			}
			lastUpdate = updatedAt
			return true
		})
	}
	for _, price := range []string{"80", "90", finalPrice} {
		setPrice(price)
		waitForQuote(price)
	}
	// Observe another scheduler cycle at the final price within the test deadline.
	waitForQuote(finalPrice)

	var active, notified bool
	var threshold string
	if err := env.DB.QueryRow(ctx, `
		SELECT is_active, is_notified, value::text FROM market_mind.alerts WHERE id = $1
	`, alert.ID).Scan(&active, &notified, &threshold); err != nil {
		t.Fatalf("read alert %d state: %v", alert.ID, err)
	}
	if !active || notified || threshold != target {
		t.Errorf("alert %d after below-target prices: active=%t, notified=%t, target=%s; want active, not notified, target=%s",
			alert.ID, active, notified, threshold, target)
	}

	// Tracking saves the quote after publishing any notification it generated.
	marketKey := strconv.FormatInt(marketID, 10)
	for _, event := range output.ReadAvailable(t, ctx) {
		for _, item := range event.Items {
			if item.AlertID == alert.ID || item.TelegramUserID == user.TelegramID || item.MarketID == marketKey {
				t.Errorf("unexpected notification for a below-target quote: %+v", item)
			}
		}
	}
	if messages := testutil.TelegramMessages(t, ctx, telegram, user.TelegramID); len(messages) != 0 {
		t.Errorf("unexpected Telegram messages for chat %d: %+v", user.TelegramID, messages)
	}
}
