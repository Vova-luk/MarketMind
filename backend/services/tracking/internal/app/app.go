package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"golang.org/x/sync/errgroup"

	"marketmind/tracking/internal/adapters/kafka/consumer"
	"marketmind/tracking/internal/adapters/kafka/producer"
	"marketmind/tracking/internal/config"
	"marketmind/tracking/internal/database"
	"marketmind/tracking/internal/repository/postgres"
	"marketmind/tracking/internal/server"
	"marketmind/tracking/internal/service"
)

type runner interface{ Run(context.Context) error }
type consumerRunner interface {
	runner
	io.Closer
}

type Application struct {
	server   runner
	consumer consumerRunner
	producer io.Closer
	pool     interface{ Close() }
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*Application, error) {
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
	pool, err := database.Connect(ctx, cfg.DB)
	if err != nil {
		return nil, fmt.Errorf("connect tracking database: %w", err)
	}
	repository := postgres.NewRepository(pool)
	publisher := producer.NewPriceChangedProducer(cfg.Kafka)
	svc := service.NewService(publisher, repository)
	reader := consumer.NewPriceCheckedConsumer(cfg.Kafka, svc, logger)

	return &Application{server: srv, consumer: reader, producer: publisher, pool: pool}, nil
}

func (a *Application) Run(ctx context.Context) error {
	group, runCtx := errgroup.WithContext(ctx)
	group.Go(func() error { return runComponent(runCtx, "HTTP server", a.server) })
	group.Go(func() error { return runComponent(runCtx, "Kafka consumer", a.consumer) })
	runErr := group.Wait()

	// The handler must finish before its producer and database are closed.
	consumerErr := a.consumer.Close()
	producerErr := a.producer.Close()
	a.pool.Close()
	if consumerErr != nil {
		consumerErr = fmt.Errorf("close Kafka consumer: %w", consumerErr)
	}
	if producerErr != nil {
		producerErr = fmt.Errorf("close Kafka producer: %w", producerErr)
	}

	return errors.Join(runErr, consumerErr, producerErr)
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
