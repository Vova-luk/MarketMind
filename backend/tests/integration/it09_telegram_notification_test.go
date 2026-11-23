//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"marketmind-tests/internal/testutil"
	"marketmind/api/pkg/alerts"
	"marketmind/contracts/events"
)

func TestIT09_TelegramNotification(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()

	stubURL := os.Getenv("TEST_TELEGRAM_STUB_URL")
	if stubURL == "" {
		t.Fatal("TEST_TELEGRAM_STUB_URL must point to the Telegram stub")
	}
	stub := testutil.NewClient(stubURL)
	kafka := testutil.NewKafka(t)
	chatID := testutil.NewTelegramID(t)
	eventID := rand.Text()
	const marketID = "1"
	kafka.PublishJSON(t, ctx, events.PriceChangedTopic, marketID, events.PriceChangedEvent{
		EventID: eventID,
		Items: []events.PriceNotification{{
			NotificationID: eventID + ":1", TelegramUserID: chatID, AlertID: 1, MarketID: marketID,
			ExchangeName: "Binance", BaseAsset: "BTC", QuoteAsset: "USDT",
			Type: alerts.Price, Direction: alerts.GreaterOrEqual,
			ReferencePrice: "90", CurrentPrice: "105", Value: "100",
		}},
	})

	var inbox struct {
		Messages []struct {
			ChatID    int64  `json:"chat_id"`
			Text      string `json:"text"`
			ParseMode string `json:"parse_mode"`
		} `json:"messages"`
	}
	path := fmt.Sprintf("/messages/%d", chatID)
	testutil.Eventually(t, ctx, fmt.Sprintf("Telegram sendMessage for chat %d", chatID), func() bool {
		stub.RequestJSON(t, ctx, http.MethodGet, path, nil, http.StatusOK, &inbox)
		return len(inbox.Messages) > 0
	})

	message := inbox.Messages[0]
	if message.ChatID != chatID || message.ParseMode != "HTML" {
		t.Fatalf("sendMessage: got chat_id=%d, parse_mode=%q, want chat_id=%d, parse_mode=HTML",
			message.ChatID, message.ParseMode, chatID)
	}
	for _, want := range []string{"BTC/USDT (Binance)", "≥ 100.00 USDT", "105.00 USDT"} {
		if !strings.Contains(message.Text, want) {
			t.Errorf("sendMessage text %q does not contain %q", message.Text, want)
		}
	}
}
