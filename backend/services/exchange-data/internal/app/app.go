package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	grpcadapter "marketmind/exchange-data/internal/adapters/inbound/grpc"
	"marketmind/exchange-data/internal/adapters/inbound/scheduler"
	"marketmind/exchange-data/internal/adapters/outbound/crypto_exchanges/binance"
	"marketmind/exchange-data/internal/adapters/outbound/crypto_exchanges/bybit"
	"marketmind/exchange-data/internal/adapters/outbound/crypto_exchanges/okx"
	"marketmind/exchange-data/internal/adapters/outbound/market_catalog"
	"marketmind/exchange-data/internal/config"
	"marketmind/exchange-data/internal/providers"
	"marketmind/exchange-data/internal/server"
	"marketmind/exchange-data/internal/service"
	"marketmind/gen/go/api"
	"marketmind/gen/go/exchangedata"
)

type runner interface{ Run(context.Context) error }

type exchangeClient interface {
	service.PriceProvider
	service.SpotProvider
}

type Application struct {
	server    runner
	grpc      runner
	scheduler runner
	kafka     io.Closer
	apiConn   io.Closer
}

func New(cfg config.Config, logger *slog.Logger) (*Application, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}

	router := http.NewServeMux()
	router.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("OK"))
	})

	srv, err := server.NewServer(cfg.Server, router, logger)
	if err != nil {
		return nil, err
	}

	clients := map[string]exchangeClient{
		"binance": binance.NewBinanceClient(cfg.Exchanges.Binance, logger),
		"bybit":   bybit.NewBybitClient(cfg.Exchanges.Bybit, logger),
		"okx":     okx.NewOKXClient(cfg.Exchanges.OKX, logger),
	}
	spotProviders := make(map[string]service.SpotProvider, len(clients))
	for name, client := range clients {
		spotProviders[name] = client
	}
	search := service.NewSearchService(spotProviders)

	application := &Application{server: srv}
	if !cfg.Scheduler.Disabled {
		exchanges, err := pollingProviders(clients, cfg.Scheduler.Exchanges)
		if err != nil {
			return nil, err
		}
		conn, err := grpc.NewClient(cfg.API.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return nil, fmt.Errorf("create API market catalog client: %w", err)
		}
		kafkaProvider, err := providers.NewKafkaProvider(cfg.Kafka)
		if err != nil {
			return nil, errors.Join(fmt.Errorf("create Kafka provider: %w", err), conn.Close())
		}
		catalog := marketcatalog.NewClient(api.NewMarketCatalogServiceClient(conn), cfg.API.RequestTimeout)
		svc := service.NewService(exchanges, kafkaProvider.PriceCheckedProducer, catalog, logger)
		loop, err := scheduler.NewScheduler(svc, cfg.Scheduler, logger)
		if err != nil {
			return nil, errors.Join(fmt.Errorf("create scheduler: %w", err), kafkaProvider.Close(), conn.Close())
		}
		application.scheduler = loop
		application.kafka = kafkaProvider
		application.apiConn = conn
	}
	grpcServer := grpc.NewServer()
	exchangedata.RegisterMarketSearchServiceServer(grpcServer, grpcadapter.NewSearchHandler(search, logger))
	application.grpc = server.NewGRPC(cfg.GRPC, grpcServer, logger)
	return application, nil
}

func pollingProviders(clients map[string]exchangeClient, configured []string) (map[string]service.PriceProvider, error) {
	exchanges := make(map[string]service.PriceProvider, len(clients))
	if configured == nil {
		for name, client := range clients {
			exchanges[name] = client
		}
		return exchanges, nil
	}
	for _, name := range configured {
		key := strings.ToLower(strings.TrimSpace(name))
		client, ok := clients[key]
		if !ok {
			return nil, fmt.Errorf("scheduler.exchanges contains unsupported exchange %q", name)
		}
		exchanges[key] = client
	}
	return exchanges, nil
}

func (a *Application) Run(ctx context.Context) error {
	group, runCtx := errgroup.WithContext(ctx)
	group.Go(func() error { return runComponent(runCtx, "HTTP server", a.server) })
	group.Go(func() error { return runComponent(runCtx, "gRPC server", a.grpc) })
	if a.scheduler != nil {
		group.Go(func() error { return runComponent(runCtx, "scheduler", a.scheduler) })
	}
	runErr := group.Wait()

	if a.kafka != nil {
		if err := a.kafka.Close(); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close Kafka provider: %w", err))
		}
	}
	if a.apiConn != nil {
		if err := a.apiConn.Close(); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close API market catalog connection: %w", err))
		}
	}

	return runErr
}

func runComponent(ctx context.Context, name string, component runner) error {
	if err := component.Run(ctx); err != nil {
		return fmt.Errorf("run %s: %w", name, err)
	}

	if ctx.Err() == nil {
		return fmt.Errorf("%s stopped unexpectedly", name)
	}

	return nil
}
