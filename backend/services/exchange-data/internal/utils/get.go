package utils

import (
	"context"
	"io"
	"log/slog"
	"net/http"
)

func Get(ctx context.Context, client *http.Client, url string, cfg RetryConfig, logger *slog.Logger, decode func(io.Reader) error) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := DoWithRetry(req, client, cfg, logger)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decode(resp.Body)
}
