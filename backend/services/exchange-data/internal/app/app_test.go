package app

import (
	"context"
	"errors"
	"testing"
	"time"
)

type runnerFunc func(context.Context) error

func (f runnerFunc) Run(ctx context.Context) error { return f(ctx) }

type closeFunc func() error

func (f closeFunc) Close() error { return f() }

func TestApplicationClosesKafkaAfterAllComponentsStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{}, 2)
	serverStopped, schedulerStopped := make(chan struct{}), make(chan struct{})
	closed := 0
	application := &Application{
		server: runnerFunc(func(ctx context.Context) error {
			defer close(serverStopped)
			started <- struct{}{}
			<-ctx.Done()
			return nil
		}),
		scheduler: runnerFunc(func(ctx context.Context) error {
			defer close(schedulerStopped)
			started <- struct{}{}
			<-ctx.Done()
			return nil
		}),
		kafka: closeFunc(func() error {
			select {
			case <-serverStopped:
			default:
				t.Error("Kafka closed before HTTP server stopped")
			}
			select {
			case <-schedulerStopped:
			default:
				t.Error("Kafka closed before in-flight price checks stopped")
			}
			closed++
			return nil
		}),
	}
	done := make(chan error, 1)
	go func() { done <- application.Run(ctx) }()
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("component did not start")
		}
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown blocked")
	}
	if closed != 1 {
		t.Fatalf("Kafka close calls=%d", closed)
	}
}

func TestApplicationFailureStopsPeerAndPreservesCloseError(t *testing.T) {
	want := errors.New("listen failed")
	closeErr := errors.New("close failed")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	peerStopped := make(chan struct{})
	application := &Application{
		server:    runnerFunc(func(context.Context) error { return want }),
		scheduler: runnerFunc(func(ctx context.Context) error { defer close(peerStopped); <-ctx.Done(); return nil }),
		kafka: closeFunc(func() error {
			select {
			case <-peerStopped:
			default:
				t.Error("peer was not stopped before close")
			}
			return closeErr
		}),
	}
	err := application.Run(ctx)
	if !errors.Is(err, want) || !errors.Is(err, closeErr) {
		t.Fatalf("expected both errors, got %v", err)
	}
	if ctx.Err() != nil {
		t.Fatal("peer stopped only because outer deadline expired")
	}
}

func TestApplicationTreatsUnexpectedComponentExitAsFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	application := &Application{
		server:    runnerFunc(func(context.Context) error { return nil }),
		scheduler: runnerFunc(func(ctx context.Context) error { <-ctx.Done(); return nil }),
		kafka:     closeFunc(func() error { return nil }),
	}
	if err := application.Run(ctx); err == nil {
		t.Fatal("unexpected component exit accepted")
	}
	if ctx.Err() != nil {
		t.Fatal("application did not cancel peer")
	}
}
