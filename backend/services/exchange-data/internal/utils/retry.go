package utils

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

type RetryConfig struct {
	MaxAttempts int
	Delay       time.Duration
}

func DoWithRetry(req *http.Request, client *http.Client, cfg RetryConfig, logger *slog.Logger) (*http.Response, error) {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Delay < 0 {
		return nil, fmt.Errorf("retry delay must not be negative")
	}
	ctx := req.Context()
	if cfg.MaxAttempts < 1 {
		return nil, fmt.Errorf("max_attempts must be at least 1")
	}
	var lastErr error

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		resp, err := client.Do(req)

		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}

			var netErr net.Error
			if !errors.As(err, &netErr) || !netErr.Timeout() {
				return nil, fmt.Errorf("send HTTP request: %w", err)
			}

			lastErr = err
		} else {
			switch resp.StatusCode {
			case http.StatusOK:
				return resp, nil

			case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
				lastErr = fmt.Errorf(
					"unexpected HTTP status %d", resp.StatusCode,
				)
				resp.Body.Close()

			default:
				resp.Body.Close()
				return nil, fmt.Errorf(
					"unexpected HTTP status %d", resp.StatusCode,
				)
			}
		}

		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if attempt == cfg.MaxAttempts {
			break
		}

		logger.WarnContext(ctx, "HTTP request failed; retrying",
			slog.Int("attempt", attempt),
			slog.Int("max_attempts", cfg.MaxAttempts),
			slog.Duration("retry_delay", cfg.Delay),
			slog.Any("error", lastErr),
		)

		timer := time.NewTimer(cfg.Delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	return nil, fmt.Errorf(
		"HTTP request failed after %d attempts: %w",
		cfg.MaxAttempts, lastErr,
	)
}
