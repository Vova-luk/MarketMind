package service

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"marketmind/api/pkg/alerts"
	"marketmind/contracts/events"
)

type recordingSender struct {
	users []int64
}

func (s *recordingSender) SendMessage(_ context.Context, userID int64, _ string) error {
	s.users = append(s.users, userID)
	return nil
}

func TestInvalidNotificationDoesNotSkipValidNotifications(t *testing.T) {
	sender := &recordingSender{}
	service := NewService(sender, 1, slog.New(slog.DiscardHandler))
	valid := events.PriceNotification{
		NotificationID: "valid", TelegramUserID: 1, BaseAsset: "BTC", QuoteAsset: "USDT",
		Type: alerts.Price, Direction: alerts.LessOrEqual, Value: "100", ReferencePrice: "105", CurrentPrice: "99",
	}
	invalid := valid
	invalid.NotificationID = "invalid"
	invalid.ReferencePrice = ""

	err := service.HandlePriceChanged(context.Background(), events.PriceChangedEvent{
		EventID: "event", Items: []events.PriceNotification{invalid, valid},
	})
	if err == nil || !strings.Contains(err.Error(), "reference_price is required") {
		t.Fatalf("expected an error for the invalid notification, got %v", err)
	}
	if len(sender.users) != 1 || sender.users[0] != valid.TelegramUserID {
		t.Fatalf("sent messages to %v, want only user %d", sender.users, valid.TelegramUserID)
	}
}
