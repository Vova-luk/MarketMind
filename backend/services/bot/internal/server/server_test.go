package server

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"marketmind/bot/internal/config"
)

func serverConfig() config.ServerConfig {
	return config.ServerConfig{
		Host: "127.0.0.1", Port: 8183,
		ReadTimeout: time.Second, WriteTimeout: 2 * time.Second, ShutdownTimeout: time.Second,
	}
}

func TestNewServerAppliesTimeoutsAndDefaultLogger(t *testing.T) {
	cfg := serverConfig()
	srv, err := NewServer(cfg, http.NewServeMux(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if srv.logger != slog.Default() || srv.Server.WriteTimeout != cfg.WriteTimeout || srv.shutdownTimeout != cfg.ShutdownTimeout {
		t.Fatal("logger or configured timeouts were not applied")
	}
	cfg.WriteTimeout = 0
	if _, err := NewServer(cfg, http.NewServeMux(), nil); err == nil {
		t.Fatal("invalid timeout was accepted")
	}
}

type logWriter struct{ lines chan string }

func (w logWriter) Write(p []byte) (int, error) {
	w.lines <- string(p)
	return len(p), nil
}

func TestRunServesAndStopsOnCancellation(t *testing.T) {
	logs := logWriter{lines: make(chan string, 16)}
	logger := slog.New(slog.NewTextHandler(logs, nil))
	router := http.NewServeMux()
	router.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("OK"))
	})
	srv, err := NewServer(serverConfig(), router, logger)
	if err != nil {
		t.Fatal(err)
	}
	srv.Server.Addr = "127.0.0.1:0"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx) }()
	var address string
	select {
	case line := <-logs.lines:
		if !strings.Contains(line, "server is running") {
			t.Fatalf("unexpected startup log: %s", line)
		}
		for _, field := range strings.Fields(line) {
			if strings.HasPrefix(field, "addr=") {
				address = strings.TrimPrefix(field, "addr=")
			}
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not start")
	}
	if address == "" {
		t.Fatal("startup log did not include the bound address")
	}
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	resp, err := client.Get("http://" + address + "/health")
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil || resp.StatusCode != http.StatusOK || string(body) != "OK" {
		t.Fatal("health endpoint failed")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not shut down")
	}
	conn, err := net.DialTimeout("tcp", address, time.Second)
	if err == nil {
		conn.Close()
		t.Fatal("listener remained open after cancellation")
	}
}

func TestRunWithCanceledContextDoesNotListen(t *testing.T) {
	srv, err := NewServer(serverConfig(), http.NewServeMux(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv.Server.Addr = "invalid address"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := srv.Run(ctx); err != nil {
		t.Fatalf("canceled server tried to listen: %v", err)
	}
}

func TestRunReturnsBindError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	srv, err := NewServer(serverConfig(), http.NewServeMux(), nil)
	if err != nil {
		t.Fatal(err)
	}
	srv.Server.Addr = listener.Addr().String()
	if err := srv.Run(context.Background()); err == nil {
		t.Fatal("expected an occupied-port error")
	}
}
