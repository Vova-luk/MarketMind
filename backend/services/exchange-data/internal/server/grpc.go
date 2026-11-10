package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	"google.golang.org/grpc"

	"marketmind/exchange-data/internal/config"
)

type GRPCServer struct {
	server          *grpc.Server
	address         string
	shutdownTimeout time.Duration
	logger          *slog.Logger
}

func NewGRPC(cfg config.GRPCConfig, server *grpc.Server, logger *slog.Logger) *GRPCServer {
	return &GRPCServer{
		server: server, address: net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		shutdownTimeout: cfg.ShutdownTimeout, logger: logger,
	}
}

func (s *GRPCServer) Run(ctx context.Context) error {
	if ctx.Err() != nil {
		return nil
	}
	listener, err := net.Listen("tcp", s.address)
	if err != nil {
		return fmt.Errorf("listen gRPC: %w", err)
	}
	defer s.server.Stop()
	served := make(chan error, 1)
	go func() { served <- s.server.Serve(listener) }()
	s.logger.InfoContext(ctx, "gRPC server is running", "addr", listener.Addr().String())

	select {
	case err := <-served:
		return err
	case <-ctx.Done():
		go s.server.GracefulStop()
	}
	timer := time.NewTimer(s.shutdownTimeout)
	defer timer.Stop()
	select {
	case err := <-served:
		// Shutdown can run before the Serve goroutine acquires the listener.
		if errors.Is(err, grpc.ErrServerStopped) {
			return nil
		}
		return err
	case <-timer.C:
		s.server.Stop()
		<-served
		return errors.New("gRPC shutdown timed out")
	}
}
