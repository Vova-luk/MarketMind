package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"marketmind/exchange-data/internal/config"
	"time"
)

type PriceChecker interface{ CheckPrices(context.Context) error }

type Scheduler struct {
	checker  PriceChecker
	interval time.Duration
	logger   *slog.Logger
}

func NewScheduler(checker PriceChecker, cfg config.SchedulerConfig, logger *slog.Logger) (*Scheduler, error) {
	if checker == nil {
		return nil, fmt.Errorf("price checker is required")
	}
	if cfg.Interval <= 0 {
		return nil, fmt.Errorf("scheduler interval must be positive")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Scheduler{checker: checker, interval: cfg.Interval, logger: logger}, nil
}

func (s *Scheduler) Run(ctx context.Context) error {
	s.logger.InfoContext(ctx, "price scheduler started", slog.Duration("interval", s.interval))
	defer s.logger.Info("price scheduler stopped")
	for {
		if ctx.Err() != nil {
			return nil
		}
		started := time.Now()
		err := s.checker.CheckPrices(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			s.logger.ErrorContext(ctx, "price check failed", slog.Any("error", err))
		} else {
			s.logger.InfoContext(ctx, "price check finished", slog.Duration("duration", time.Since(started)))
		}
		timer := time.NewTimer(s.interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}
