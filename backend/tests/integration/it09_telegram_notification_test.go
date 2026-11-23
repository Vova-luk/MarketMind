//go:build integration

package integration

import (
	"context"
	"crypto/rand"
	"fmt"
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

	stub := testutil.ClientFromEnv(t, "TEST_TELEGRAM_STUB_URL")
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

	var messages []testutil.TelegramMessage
	testutil.Eventually(t, ctx, fmt.Sprintf("Telegram sendMessage for chat %d", chatID), func() bool {
		messages = testutil.TelegramMessages(t, ctx, stub, chatID)
		return len(messages) > 0
	})

	message := messages[0]
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
