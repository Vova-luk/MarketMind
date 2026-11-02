package application

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"testing"
	"time"

	"marketmind/api/internal/server"
)

func TestShutdownRunsInReverseOrderAndContinuesAfterErrors(t *testing.T) {
	var order []string
	wantErr := errors.New("server shutdown failed")
	a := &Application{shutdownFuncs: []func() error{
		func() error { order = append(order, "logger"); return nil },
		func() error { order = append(order, "database"); return nil },
		func() error { order = append(order, "server"); return wantErr },
	}}
	if err := a.shutdown(); !errors.Is(err, wantErr) {
		t.Fatalf("expected shutdown error, got %v", err)
	}
	if !reflect.DeepEqual(order, []string{"server", "database", "logger"}) {
		t.Fatalf("unexpected shutdown order: %v", order)
	}
}

func TestRunShutsDownOnListenError(t *testing.T) {
	closed := false
	a := &Application{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		server: &server.Server{Server: &http.Server{Addr: "invalid address"}},
		shutdownFuncs: []func() error{func() error {
			closed = true
			return nil
		}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := a.Run(ctx); err == nil {
		t.Fatal("expected listen error")
	}
	if !closed {
		t.Fatal("resources were not closed after listen error")
	}
}
