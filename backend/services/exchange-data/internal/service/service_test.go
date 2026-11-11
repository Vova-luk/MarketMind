package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"marketmind/contracts/events"
	"marketmind/exchange-data/internal/domain"
)

type sourceFunc func(context.Context) ([]ExchangeMarkets, error)

func (f sourceFunc) GetMarketsToCheck(ctx context.Context) ([]ExchangeMarkets, error) { return f(ctx) }

type providerFunc func(context.Context, []string) (*domain.Market, error)

func (f providerFunc) GetPrices(ctx context.Context, symbols []string) (*domain.Market, error) {
	return f(ctx, symbols)
}

type publisherFunc func(context.Context, events.PricesCheckedEvent) error

func (f publisherFunc) SendPriceChecked(ctx context.Context, event events.PricesCheckedEvent) error {
	return f(ctx, event)
}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestCheckPricesMapsSymbolsAndKeepsPartialSuccess(t *testing.T) {
	wantTime := time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)
	input := []ExchangeMarkets{
		{ExchangeName: " Binance ", Markets: []MarketToCheck{{"42", " BTCUSDT "}, {"43", "ETHUSDT"}, {"45", "MISSING"}}},
		{ExchangeName: "bybit", Markets: []MarketToCheck{{"46", "BTCUSDT"}}},
		{ExchangeName: "okx", Markets: []MarketToCheck{{"44", "BTC-USDT"}}},
		{ExchangeName: "unknown", Markets: []MarketToCheck{{"47", "BTCUSDT"}}},
	}
	sourceCalls := 0
	providers := map[string]PriceProvider{
		"binance": providerFunc(func(_ context.Context, symbols []string) (*domain.Market, error) {
			if !reflect.DeepEqual(symbols, []string{"BTCUSDT", "ETHUSDT", "MISSING"}) {
				t.Errorf("unexpected symbols: %v", symbols)
			}
			// Providers normalize in place; the original source must not be changed.
			symbols[0] = "changed"
			return &domain.Market{ReceivedAt: wantTime, Cryptocurrency: []domain.Cryptocurrency{
				{Symbol: "ETHUSDT", Price: "invalid", Error: "bad price"},
				{Symbol: "EXTRA", Price: "999"},
				{Symbol: "BTCUSDT", Price: "65000.50000000"},
			}}, nil
		}),
		"bybit": providerFunc(func(context.Context, []string) (*domain.Market, error) {
			return nil, errors.New("exchange unavailable")
		}),
		"okx": providerFunc(func(context.Context, []string) (*domain.Market, error) {
			return &domain.Market{ReceivedAt: wantTime, Cryptocurrency: []domain.Cryptocurrency{{Symbol: "BTC-USDT", Price: "65001.00"}}}, nil
		}),
	}
	var published events.PricesCheckedEvent
	calls := 0
	ctx := context.Background()
	svc := NewService(providers, publisherFunc(func(got context.Context, event events.PricesCheckedEvent) error {
		if got != ctx {
			t.Error("publisher lost caller context")
		}
		calls++
		published = event
		return nil
	}), sourceFunc(func(context.Context) ([]ExchangeMarkets, error) { sourceCalls++; return input, nil }), quietLogger())
	if err := svc.CheckPrices(ctx); err != nil {
		t.Fatal(err)
	}
	want := []events.CheckedPrice{
		{ExchangeName: "binance", MarketID: "42", ExternalID: "BTCUSDT", Price: "65000.50000000", ReceivedAt: wantTime},
		{ExchangeName: "okx", MarketID: "44", ExternalID: "BTC-USDT", Price: "65001.00", ReceivedAt: wantTime},
	}
	if calls != 1 || sourceCalls != 1 || published.EventID == "" || !reflect.DeepEqual(published.Items, want) {
		t.Fatalf("calls=%d, source=%d, event=%+v", calls, sourceCalls, published)
	}
	if input[0].Markets[0].ExternalID != " BTCUSDT " {
		t.Error("source data was mutated")
	}
}

func TestCheckPricesReturnsSourceError(t *testing.T) {
	want := errors.New("API unavailable")
	svc := NewService(nil, publisherFunc(func(context.Context, events.PricesCheckedEvent) error { t.Fatal("unexpected publication"); return nil }), sourceFunc(func(context.Context) ([]ExchangeMarkets, error) { return nil, want }), nil)
	if err := svc.CheckPrices(context.Background()); !errors.Is(err, want) {
		t.Fatalf("source error lost: %v", err)
	}
}

func TestCheckPricesWithNoScheduledMarkets(t *testing.T) {
	for _, items := range [][]ExchangeMarkets{nil, {}, {{ExchangeName: "binance"}}} {
		svc := NewService(nil, publisherFunc(func(context.Context, events.PricesCheckedEvent) error { t.Fatal("empty event published"); return nil }), sourceFunc(func(context.Context) ([]ExchangeMarkets, error) { return items, nil }), quietLogger())
		if err := svc.CheckPrices(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCheckPricesFailsWhenNoValidPricesRemain(t *testing.T) {
	want := errors.New("provider unavailable")
	for _, tt := range []struct {
		name   string
		prices *domain.Market
		err    error
	}{
		{"provider error", nil, want},
		{"nil market", nil, nil},
		{"empty prices", &domain.Market{}, nil},
		{"invalid quote", &domain.Market{Cryptocurrency: []domain.Cryptocurrency{{Symbol: "BTCUSDT", Error: "bad quote"}}}, nil},
		{"empty price", &domain.Market{Cryptocurrency: []domain.Cryptocurrency{{Symbol: "BTCUSDT"}}}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(map[string]PriceProvider{"binance": providerFunc(func(context.Context, []string) (*domain.Market, error) { return tt.prices, tt.err })}, publisherFunc(func(context.Context, events.PricesCheckedEvent) error {
				t.Fatal("no valid prices must not be published")
				return nil
			}), sourceFunc(func(context.Context) ([]ExchangeMarkets, error) {
				return []ExchangeMarkets{{"binance", []MarketToCheck{{"42", "BTCUSDT"}}}}, nil
			}), quietLogger())
			err := svc.CheckPrices(context.Background())
			if err == nil {
				t.Fatal("expected failure")
			}
			if tt.err != nil && !errors.Is(err, tt.err) {
				t.Fatalf("provider error lost: %v", err)
			}
		})
	}
}

func TestCheckPricesReturnsPublishError(t *testing.T) {
	want := errors.New("Kafka unavailable")
	calls := 0
	svc := NewService(map[string]PriceProvider{"binance": providerFunc(func(context.Context, []string) (*domain.Market, error) {
		return &domain.Market{Cryptocurrency: []domain.Cryptocurrency{{Symbol: "BTCUSDT", Price: "1"}}}, nil
	})}, publisherFunc(func(context.Context, events.PricesCheckedEvent) error { calls++; return want }), sourceFunc(func(context.Context) ([]ExchangeMarkets, error) {
		return []ExchangeMarkets{{"binance", []MarketToCheck{{"42", "BTCUSDT"}}}}, nil
	}), quietLogger())
	if err := svc.CheckPrices(context.Background()); !errors.Is(err, want) || calls != 1 {
		t.Fatalf("err=%v, calls=%d", err, calls)
	}
}

func TestCheckPricesUsesAtMostThreeConcurrentProviders(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	gate := make(chan struct{})
	var once sync.Once
	defer once.Do(func() { close(gate) })
	started := make(chan struct{}, 6)
	var active, maximum atomic.Int32
	var items []ExchangeMarkets
	providers := make(map[string]PriceProvider)
	for i := 0; i < 6; i++ {
		name := fmt.Sprintf("exchange-%d", i)
		items = append(items, ExchangeMarkets{name, []MarketToCheck{{fmt.Sprint(i), "BTCUSDT"}}})
		providers[name] = providerFunc(func(ctx context.Context, _ []string) (*domain.Market, error) {
			n := active.Add(1)
			defer active.Add(-1)
			for {
				old := maximum.Load()
				if n <= old || maximum.CompareAndSwap(old, n) {
					break
				}
			}
			started <- struct{}{}
			select {
			case <-gate:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			return &domain.Market{Cryptocurrency: []domain.Cryptocurrency{{Symbol: "BTCUSDT", Price: "1"}}}, nil
		})
	}
	svc := NewService(providers, publisherFunc(func(context.Context, events.PricesCheckedEvent) error { return nil }), sourceFunc(func(context.Context) ([]ExchangeMarkets, error) { return items, nil }), quietLogger())
	done := make(chan error, 1)
	go func() { done <- svc.CheckPrices(ctx) }()
	for i := 0; i < 3; i++ {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("three providers did not start concurrently")
		}
	}
	once.Do(func() { close(gate) })
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("check did not finish")
	}
	if maximum.Load() != 3 {
		t.Fatalf("maximum active requests=%d", maximum.Load())
	}
}

func TestCheckPricesCancellationPreventsPublication(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	svc := NewService(map[string]PriceProvider{"binance": providerFunc(func(ctx context.Context, _ []string) (*domain.Market, error) { cancel(); return nil, ctx.Err() })}, publisherFunc(func(context.Context, events.PricesCheckedEvent) error {
		t.Fatal("canceled cycle published an event")
		return nil
	}), sourceFunc(func(context.Context) ([]ExchangeMarkets, error) {
		return []ExchangeMarkets{{"binance", []MarketToCheck{{"42", "BTCUSDT"}}}}, nil
	}), quietLogger())
	if err := svc.CheckPrices(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation: %v", err)
	}
}
