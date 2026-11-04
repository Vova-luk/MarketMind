package app

import (
	"context"
	"log/slog"
	"net/http"

	"marketmind/bot/internal/config"
	"marketmind/bot/internal/server"
)

type Application struct {
	server *server.Server
}

func New(cfg config.Config, logger *slog.Logger) (*Application, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
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
	return &Application{server: srv}, nil
}

func (a *Application) Run(ctx context.Context) error {
	return a.server.Run(ctx)
}
