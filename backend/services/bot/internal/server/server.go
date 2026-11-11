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

	"marketmind/bot/internal/config"
)

const (
	addressLogKey  = "addr"
	serverRunning  = "server is running"
	serverStopping = "shutting down server"
	serverStopped  = "server exited cleanly"
)

type Server struct {
	Server          *http.Server
	logger          *slog.Logger
	shutdownTimeout time.Duration
}

func NewServer(cfg config.ServerConfig, router http.Handler, logger *slog.Logger) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		Server: &http.Server{
			Addr:         net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
			Handler:      router,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError),
		},
		logger:          logger,
		shutdownTimeout: cfg.ShutdownTimeout,
	}, nil
}

func (s *Server) Run(ctx context.Context) error {
	if ctx.Err() != nil {
		return nil
	}
	listener, err := net.Listen("tcp", s.Server.Addr)
	if err != nil {
		return fmt.Errorf("listen HTTP: %w", err)
	}
	addr := listener.Addr().String()
	errChan := make(chan error, 1)
	go func() {
		err := s.Server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errChan <- err
	}()
	s.logger.InfoContext(ctx, serverRunning, slog.String(addressLogKey, addr))

	select {
	case <-ctx.Done():
		return s.shutdown(errChan)
	case err := <-errChan:
		if err != nil {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	}
}

func (s *Server) shutdown(errChan <-chan error) error {
	ctx, cancel := context.WithTimeout(context.Background(), s.shutdownTimeout)
	defer cancel()
	s.logger.Info(serverStopping)
	shutdownErr := s.Server.Shutdown(ctx)
	if shutdownErr != nil {
		shutdownErr = errors.Join(fmt.Errorf("shutdown HTTP: %w", shutdownErr), s.Server.Close())
	}
	serveErr := <-errChan
	if err := errors.Join(shutdownErr, serveErr); err != nil {
		return err
	}
	s.logger.Info(serverStopped)
	return nil
}
