package application

import (
	"context"
	"testing"

	domain "marketmind/api/internal/domain/alerts"
	alertservice "marketmind/api/internal/service/alerts"
)

func TestUpdateAlertNotificationState(t *testing.T) {
	active, inactive := true, false
	for _, tc := range []struct {
		name         string
		condition    *domain.Condition
		active       *bool
		wasActive    bool
		wantNotified bool
	}{
		{"value changed", &domain.Condition{Type: "percent", Value: "10", Direction: domain.GreaterOrEqual}, nil, true, false},
		{"type changed", &domain.Condition{Type: "price", Value: "120", Direction: domain.GreaterOrEqual}, nil, true, false},
		{"direction changed", &domain.Condition{Type: "percent", Value: "5", Direction: domain.LessOrEqual}, nil, true, false},
		{"equivalent value", &domain.Condition{Type: "percent", Value: "5.0", Direction: domain.GreaterOrEqual}, nil, true, true},
		{"re-enabled", nil, &active, false, false},
		{"already enabled", nil, &active, true, true},
		{"disabled", nil, &inactive, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &alertStoreStub{
				subscription: domain.Subscription{Price: "110", QuoteAsset: "USDT", ExchangeActive: true},
				alert: domain.Alert{ID: 9, Condition: domain.Condition{Type: "percent", Value: "5", Direction: domain.GreaterOrEqual},
					ReferencePrice: "100", IsActive: tc.wasActive, IsNotified: true},
			}
			_, err := alertservice.NewService(store, 50).Update(context.Background(), 123, 18, 9,
				alertservice.Patch{Condition: tc.condition, IsActive: tc.active})
			if err != nil {
				t.Fatal(err)
			}
			if store.alert.IsNotified != tc.wantNotified {
				t.Fatalf("is_notified = %v, want %v", store.alert.IsNotified, tc.wantNotified)
			}
		})
	}
}
