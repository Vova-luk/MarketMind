package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"golang.org/x/sync/errgroup"

	"marketmind/bot/internal/adapters/kafka/consumer"
	"marketmind/bot/internal/adapters/telegram"
	"marketmind/bot/internal/config"
	"marketmind/bot/internal/server"
	"marketmind/bot/internal/service"
)

const (
	healthRoute       = "GET /health"
	healthContentType = "text/plain; charset=utf-8"
	healthResponse    = "OK"
)

type Application struct {
	server   *server.Server
	consumer *consumer.PriceChangedConsumer
}

func New(cfg config.Config, logger *slog.Logger) (*Application, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	router := http.NewServeMux()
	router.HandleFunc(healthRoute, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", healthContentType)
		_, _ = w.Write([]byte(healthResponse))
	})
	srv, err := server.NewServer(cfg.Server, router, logger)
	if err != nil {
		return nil, err
	}
	client, err := telegram.NewClient(cfg.Telegram)
	if err != nil {
		return nil, err
	}
	handler := service.NewService(client, cfg.Kafka.Workers, logger)
	return &Application{
		server:   srv,
		consumer: consumer.NewPriceChangedConsumer(cfg.Kafka, handler, logger),
	}, nil
}

func (a *Application) Run(ctx context.Context) error {
	group, runCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		if err := a.server.Run(runCtx); err != nil {
			return fmt.Errorf("run HTTP server: %w", err)
		}
		return nil
	})
	group.Go(func() error { return a.consumer.Run(runCtx) })
	return group.Wait()
}
