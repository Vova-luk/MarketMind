package scheduler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"marketmind/exchange-data/internal/config"
)

type checkerFunc func(context.Context) error

func (f checkerFunc) CheckPrices(ctx context.Context) error { return f(ctx) }

func TestSchedulerRunsImmediatelyAndContinuesAfterError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var calls int
	var logs strings.Builder
	loop, err := NewScheduler(checkerFunc(func(context.Context) error {
		calls++
		if calls == 1 {
			return errors.New("temporary check failure")
		}
		cancel()
		return nil
	}), config.SchedulerConfig{Interval: 5 * time.Millisecond}, slog.New(slog.NewTextHandler(&logs, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if err := loop.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || !strings.Contains(logs.String(), "price check failed") {
		t.Fatalf("calls=%d, logs=%s", calls, logs.String())
	}
}

func TestSchedulerWaitsAfterCompletionAndDoesNotOverlap(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	interval := 20 * time.Millisecond
	var active atomic.Int32
	var completed time.Time
	calls := 0
	loop, err := NewScheduler(checkerFunc(func(context.Context) error {
		if active.Add(1) != 1 {
			t.Error("checks overlap")
		}
		defer active.Add(-1)
		calls++
		if calls == 1 {
			completed = time.Now()
			return nil
		}
		if time.Since(completed) < interval {
			t.Error("next check started before full post-check interval")
		}
		cancel()
		return nil
	}), config.SchedulerConfig{Interval: interval}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	if err := loop.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestSchedulerCancellationInterruptsCheck(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{})
	loop, err := NewScheduler(checkerFunc(func(got context.Context) error {
		close(started)
		<-got.Done()
		return got.Err()
	}), config.SchedulerConfig{Interval: time.Hour}, nil)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- loop.Run(ctx) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first check did not start immediately")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("check did not stop")
	}
}

func TestSchedulerCancellationInterruptsWait(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan struct{}, 1)
	loop, err := NewScheduler(checkerFunc(func(context.Context) error { finished <- struct{}{}; return nil }), config.SchedulerConfig{Interval: time.Hour}, nil)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- loop.Run(ctx) }()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("first check did not run")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("retry interval blocked cancellation")
	}
}

func TestSchedulerDoesNotRunCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	loop, err := NewScheduler(checkerFunc(func(context.Context) error { t.Fatal("check ran after cancellation"); return nil }), config.SchedulerConfig{Interval: time.Second}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := loop.Run(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestSchedulerRejectsInvalidSettings(t *testing.T) {
	for _, interval := range []time.Duration{0, -time.Second} {
		if _, err := NewScheduler(checkerFunc(func(context.Context) error { return nil }), config.SchedulerConfig{Interval: interval}, nil); err == nil {
			t.Error("invalid interval accepted")
		}
	}
	if _, err := NewScheduler(nil, config.SchedulerConfig{Interval: time.Second}, nil); err == nil {
		t.Error("nil checker accepted")
	}
}
