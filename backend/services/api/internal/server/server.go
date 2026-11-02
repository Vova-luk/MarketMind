package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"
)

type Config struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	ReadTimeout     time.Duration `yaml:"read_timeout"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

func (c Config) Validate() error {
	if c.Host == "" {
		return errors.New("server.host is required")
	}
	if c.Port < 1 || c.Port > 65535 {
		return errors.New("server.port must be between 1 and 65535")
	}
	if c.ReadTimeout <= 0 || c.ShutdownTimeout <= 0 {
		return errors.New("server timeouts must be positive")
	}
	return nil
}

type Server struct {
	*http.Server
	shutdownTimeout time.Duration
	logger          *slog.Logger
}

func New(cfg Config, router http.Handler, logger *slog.Logger) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Server{Server: &http.Server{
		Addr:        net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Handler:     router,
		ReadTimeout: cfg.ReadTimeout,
		ErrorLog:    slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}, shutdownTimeout: cfg.ShutdownTimeout, logger: logger}, nil
}

func (s *Server) Shutdown() error {
	s.logger.Info("HTTP server stopping")
	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()
	if err := s.Server.Shutdown(ctx); err != nil {
		return errors.Join(fmt.Errorf("shutdown HTTP: %w", err), s.Close())
	}
	s.logger.Info("HTTP server stopped")
	return nil
}
