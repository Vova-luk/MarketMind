package app

import (
	"context"
	"log/slog"
	"net/http"

	"marketmind/exchange-data/internal/adapters/outbound/crypto_exchanges/binance"
	"marketmind/exchange-data/internal/adapters/outbound/crypto_exchanges/bybit"
	"marketmind/exchange-data/internal/adapters/outbound/crypto_exchanges/okx"
	"marketmind/exchange-data/internal/config"
	"marketmind/exchange-data/internal/server"
)

type Application struct {
	server        *server.Server
	okxClient     *okx.OKXClient
	bybitClient   *bybit.BybitClient
	binanceClient *binance.BinanceClient
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
	return &Application{
		server:        srv,
		binanceClient: binance.NewBinanceClient(cfg.Exchanges.Binance, logger),
		bybitClient:   bybit.NewBybitClient(cfg.Exchanges.Bybit, logger),
		okxClient:     okx.NewOKXClient(cfg.Exchanges.OKX, logger),
	}, nil
}

func (a *Application) Run(ctx context.Context) error {
	return a.server.Run(ctx)
}
