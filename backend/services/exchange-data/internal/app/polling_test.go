package app

import (
	"context"
	"io"
	"log/slog"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"marketmind/contracts/events"
	"marketmind/exchange-data/internal/adapters/inbound/scheduler"
	"marketmind/exchange-data/internal/adapters/outbound/market_catalog"
	"marketmind/exchange-data/internal/config"
	"marketmind/exchange-data/internal/domain"
	"marketmind/exchange-data/internal/service"
	"marketmind/gen/go/api"
)

type pollingCatalog struct {
	api.UnimplementedMarketCatalogServiceServer
	calls atomic.Int32
}

func (c *pollingCatalog) GetMarketsToCheck(context.Context, *api.GetMarketsToCheckRequest) (*api.GetMarketsToCheckResponse, error) {
	var market *api.MarketToCheck
	switch c.calls.Add(1) {
	case 1:
		return nil, status.Error(codes.Unavailable, "API temporarily unavailable")
	case 2:
		market = &api.MarketToCheck{MarketId: "9223372036854775807", ExternalId: "BTCUSDT"}
	case 3:
		return &api.GetMarketsToCheckResponse{}, nil
	case 4:
		market = &api.MarketToCheck{MarketId: "70", ExternalId: "ETHUSDT"}
	default:
		return &api.GetMarketsToCheckResponse{}, nil
	}
	return &api.GetMarketsToCheckResponse{Items: []*api.ExchangeMarkets{
		{ExchangeName: " Binance ", Markets: []*api.MarketToCheck{market}},
		{ExchangeName: "OKX", Markets: []*api.MarketToCheck{{MarketId: "42", ExternalId: "BTC-USDT"}}},
	}}, nil
}

type pollingPriceClient struct {
	service.SpotProvider
	getPrices func(context.Context, []string) (*domain.Market, error)
}

func (c pollingPriceClient) GetPrices(ctx context.Context, symbols []string) (*domain.Market, error) {
	return c.getPrices(ctx, symbols)
}

type recordingPublisher chan events.PricesCheckedEvent

func (p recordingPublisher) SendPriceChecked(ctx context.Context, event events.PricesCheckedEvent) error {
	select {
	case p <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestSchedulerPollsAPICatalogEveryCycle(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	api.RegisterMarketCatalogServiceServer(grpcServer, &pollingCatalog{})
	go func() { _ = grpcServer.Serve(listener) }()
	t.Cleanup(grpcServer.Stop)

	conn, err := grpc.NewClient("passthrough:///market-catalog", grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return listener.DialContext(ctx) }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	clients := map[string]exchangeClient{
		"binance": pollingPriceClient{getPrices: func(_ context.Context, symbols []string) (*domain.Market, error) {
			if len(symbols) != 1 || (symbols[0] != "BTCUSDT" && symbols[0] != "ETHUSDT") {
				t.Errorf("unexpected Binance symbols %v", symbols)
				return &domain.Market{}, nil
			}
			return &domain.Market{Cryptocurrency: []domain.Cryptocurrency{{Symbol: symbols[0], Price: "100.01"}}}, nil
		}},
		"okx": pollingPriceClient{getPrices: func(context.Context, []string) (*domain.Market, error) {
			t.Error("exchange excluded by scheduler.exchanges was polled")
			return &domain.Market{}, nil
		}},
	}
	exchanges, err := pollingProviders(clients, []string{"Binance"})
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	published := make(recordingPublisher, 4)
	catalog := marketcatalog.NewClient(api.NewMarketCatalogServiceClient(conn), time.Second)
	checker := service.NewService(exchanges, published, catalog, logger)
	loop, err := scheduler.NewScheduler(checker, config.SchedulerConfig{Interval: 10 * time.Millisecond}, logger)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	done := make(chan error, 1)
	go func() { done <- loop.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})

	for _, want := range []struct{ id, symbol string }{{"9223372036854775807", "BTCUSDT"}, {"70", "ETHUSDT"}} {
		select {
		case event := <-published:
			if len(event.Items) != 1 || event.Items[0].ExchangeName != "binance" ||
				event.Items[0].MarketID != want.id || event.Items[0].ExternalID != want.symbol || event.Items[0].Price != "100.01" {
				t.Fatalf("unexpected polling event: %+v", event)
			}
		case <-ctx.Done():
			t.Fatalf("no event for %s: %v", want.symbol, ctx.Err())
		}
	}
}
