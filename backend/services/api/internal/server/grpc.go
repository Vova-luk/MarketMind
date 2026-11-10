package server

import (
	"errors"
	"net"
	"strconv"
	"time"

	"google.golang.org/grpc"
)

type GRPCConfig struct {
	Host            string        `yaml:"host"`
	Port            int           `yaml:"port"`
	ShutdownTimeout time.Duration `yaml:"shutdown_timeout"`
}

type GRPCServer struct {
	*grpc.Server
	Addr            string
	shutdownTimeout time.Duration
}

func NewGRPC(cfg GRPCConfig) (*GRPCServer, error) {
	if cfg.Host == "" {
		return nil, errors.New("grpc.host is required")
	}
	if cfg.Port < 1 || cfg.Port > 65535 {
		return nil, errors.New("grpc.port must be between 1 and 65535")
	}
	if cfg.ShutdownTimeout <= 0 {
		return nil, errors.New("grpc.shutdown_timeout must be positive")
	}
	return &GRPCServer{
		Server:          grpc.NewServer(),
		Addr:            net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		shutdownTimeout: cfg.ShutdownTimeout,
	}, nil
}

func (s *GRPCServer) ListenAndServe() error {
	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	return s.Serve(listener)
}

func (s *GRPCServer) Shutdown() error {
	done := make(chan struct{})
	timer := time.NewTimer(s.shutdownTimeout)
	defer timer.Stop()
	go func() {
		s.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-timer.C:
		s.Stop()
		return errors.New("gRPC shutdown timed out")
	}
}
