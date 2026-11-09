package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"golang.org/x/sync/errgroup"

	"marketmind/exchange-data/internal/adapters/inbound/scheduler"
	"marketmind/exchange-data/internal/adapters/outbound/crypto_exchanges/binance"
	"marketmind/exchange-data/internal/adapters/outbound/crypto_exchanges/bybit"
	"marketmind/exchange-data/internal/adapters/outbound/crypto_exchanges/okx"
	"marketmind/exchange-data/internal/config"
	"marketmind/exchange-data/internal/providers"
	"marketmind/exchange-data/internal/server"
	"marketmind/exchange-data/internal/service"
)

type runner interface{ Run(context.Context) error }

type Application struct {
	server    runner
	scheduler runner
	kafka     io.Closer
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

	kafkaProvider, err := providers.NewKafkaProvider(cfg.Kafka)
	if err != nil {
		return nil, fmt.Errorf("create Kafka provider: %w", err)
	}

	exchanges := map[string]service.PriceProvider{
		"binance": binance.NewBinanceClient(cfg.Exchanges.Binance, logger),
		"bybit":   bybit.NewBybitClient(cfg.Exchanges.Bybit, logger),
		"okx":     okx.NewOKXClient(cfg.Exchanges.OKX, logger),
	}

	svc := service.NewService(exchanges, kafkaProvider.PriceCheckedProducer, &service.StubMarketSource{}, logger)
	loop, err := scheduler.NewScheduler(svc, cfg.Scheduler, logger)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("create scheduler: %w", err), kafkaProvider.Close())
	}
	return &Application{server: srv, scheduler: loop, kafka: kafkaProvider}, nil
}

func (a *Application) Run(ctx context.Context) error {
	group, runCtx := errgroup.WithContext(ctx)
	group.Go(func() error { return runComponent(runCtx, "HTTP server", a.server) })
	group.Go(func() error { return runComponent(runCtx, "scheduler", a.scheduler) })
	runErr := group.Wait()

	if err := a.kafka.Close(); err != nil {
		return errors.Join(runErr, fmt.Errorf("close Kafka provider: %w", err))
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
