package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"marketmind/api/pkg/alerts"
	"marketmind/contracts/events"
	"marketmind/tracking/internal/domain"
)

type repositoryStub struct {
	market  domain.Market
	alert   domain.Alert
	states  map[int64]bool
	updated bool
}

func (r *repositoryStub) GetMarket(context.Context, int64) (domain.Market, error) {
	return r.market, nil
}
func (r *repositoryStub) GetActiveAlerts(context.Context, int64) ([]domain.Alert, error) {
	return []domain.Alert{r.alert}, nil
}
func (r *repositoryStub) SetAlertNotified(_ context.Context, id int64, value bool) error {
	r.states[id] = value
	return nil
}
func (r *repositoryStub) UpdateMarketPrice(context.Context, events.CheckedPrice) error {
	r.updated = true
	return nil
}

type producerStub struct {
	items []events.PriceNotification
	err   error
}

func (p *producerStub) SendPriceChangedEvent(_ context.Context, e events.PriceChangedEvent) error {
	p.items = append(p.items, e.Items...)
	return p.err
}

func fixture() (*repositoryStub, *producerStub, events.PricesCheckedEvent) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	r := &repositoryStub{
		market: domain.Market{ID: 42, ExchangeName: "Binance", ExternalID: "BTCUSDT", ExchangeActive: true, Price: "99", UpdatedAt: now, BaseAsset: "BTC", QuoteAsset: "USDT"},
		alert:  domain.Alert{AlertID: 7, MarketID: 42, TelegramUserID: 123, Type: alerts.Price, Value: "100", Direction: alerts.GreaterOrEqual, ReferencePrice: "98", NotificationEnabled: true}, states: map[int64]bool{},
	}
	e := events.PricesCheckedEvent{EventID: "event-1", Items: []events.CheckedPrice{{MarketID: "42", ExchangeName: "binance", ExternalID: "BTCUSDT", Price: "100", ReceivedAt: now.Add(time.Second)}}}
	return r, &producerStub{}, e
}

func TestNotifications(t *testing.T) {
	for _, name := range []string{"crossing", "already notified", "no crossing", "rearm", "percent", "disabled", "precision", "stale"} {
		t.Run(name, func(t *testing.T) {
			r, p, e := fixture()
			want := 0
			switch name {
			case "crossing":
				want = 1
			case "already notified":
				r.alert.IsNotified = true
			case "no crossing":
				r.market.Price = "101"
			case "rearm":
				r.alert.IsNotified = true
				e.Items[0].Price = "99"
			case "percent":
				r.alert.Type = alerts.Percent
				r.alert.Value = "-5"
				r.alert.ReferencePrice = "100"
				r.alert.Direction = alerts.LessOrEqual
				e.Items[0].Price = "95"
				want = 1
			case "disabled":
				r.alert.NotificationEnabled = false
			case "precision":
				r.market.Price = "0.1"
				r.alert.Value = "0.100000000000000001"
				e.Items[0].Price = r.alert.Value
				want = 1
			case "stale":
				e.Items[0].ReceivedAt = r.market.UpdatedAt
			}
			if err := NewService(p, r).HandlePricesChecked(context.Background(), e); err != nil {
				t.Fatal(err)
			}
			if len(p.items) != want {
				t.Fatalf("got %d notifications, want %d", len(p.items), want)
			}
			if want > 0 && !r.states[7] {
				t.Fatal("notification flag not saved")
			}
			if want > 0 && p.items[0].ReferencePrice != r.alert.ReferencePrice {
				t.Fatalf("reference price = %q, want %q", p.items[0].ReferencePrice, r.alert.ReferencePrice)
			}
			if name == "rearm" {
				if state, ok := r.states[7]; !ok || state {
					t.Fatal("alert was not rearmed")
				}
			}
			if name == "stale" && r.updated {
				t.Fatal("stale price saved")
			}
		})
	}
}

func TestPublishFailureDoesNotMarkNotifiedOrSavePrice(t *testing.T) {
	r, p, e := fixture()
	want := errors.New("Kafka unavailable")
	p.err = want
	if err := NewService(p, r).HandlePricesChecked(context.Background(), e); !errors.Is(err, want) {
		t.Fatalf("got %v", err)
	}
	if len(r.states) > 0 || r.updated {
		t.Fatal("saved state despite publish failure")
	}
}

func TestInvalidPrice(t *testing.T) {
	for _, value := range []string{"abc", "NaN", "0", "-1", "1/2"} {
		r, p, e := fixture()
		e.Items[0].Price = value
		if err := NewService(p, r).HandlePricesChecked(context.Background(), e); err == nil {
			t.Fatalf("accepted %q", value)
		}
		if len(p.items) > 0 || r.updated {
			t.Fatal("invalid price processed")
		}
	}
}
