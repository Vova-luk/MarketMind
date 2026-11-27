package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

const pollTimeout = 30 * time.Second

type Poller struct {
	client  *Client
	handler *Handler
	logger  *slog.Logger
}

func NewPoller(client *Client, handler *Handler, logger *slog.Logger) *Poller {
	if logger == nil {
		logger = slog.Default()
	}
	return &Poller{client: client, handler: handler, logger: logger}
}

func (c *Client) getUpdates(ctx context.Context, offset int64) ([]Update, error) {
	request := struct {
		Offset         int64    `json:"offset"`
		Timeout        int      `json:"timeout"`
		AllowedUpdates []string `json:"allowed_updates"`
	}{Offset: offset, Timeout: int(pollTimeout / time.Second), AllowedUpdates: []string{"message"}}
	var updates []Update
	if err := c.call(ctx, c.pollHTTP, "getUpdates", request, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

func (p *Poller) Run(ctx context.Context) error {
	p.logger.InfoContext(ctx, "Telegram command polling started")
	var offset int64
	delay := time.Second
	for ctx.Err() == nil {
		err := p.poll(ctx, &offset)
		if ctx.Err() != nil {
			return nil
		}
		if err == nil {
			delay = time.Second
			continue
		}
		var apiErr *apiError
		if errors.As(err, &apiErr) && apiErr.method == "getUpdates" &&
			(apiErr.code == http.StatusUnauthorized || apiErr.code == http.StatusConflict) {
			return fmt.Errorf("Telegram polling failed (check token, webhook and other polling instances): %w", err)
		}
		wait := delay
		if errors.As(err, &apiErr) {
			wait = max(wait, apiErr.RetryAfter())
		}
		p.logger.WarnContext(ctx, "Telegram polling failed, retrying", "error", err, "retry_in", wait)
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
		delay = min(2*delay, 30*time.Second)
	}
	return nil
}

func (p *Poller) poll(ctx context.Context, offset *int64) error {
	updates, err := p.client.getUpdates(ctx, *offset)
	if err != nil {
		return err
	}
	for _, update := range updates {
		if update.ID < *offset {
			continue
		}
		if err := p.handler.HandleUpdate(ctx, update); err != nil {
			var apiErr *apiError
			if !errors.As(err, &apiErr) || (apiErr.code != http.StatusBadRequest && apiErr.code != http.StatusForbidden) {
				return fmt.Errorf("handle Telegram update %d: %w", update.ID, err)
			}
			// A blocked chat or rejected message must not hold up other users.
			p.logger.WarnContext(ctx, "Telegram command reply rejected", "update_id", update.ID, "error", err)
		}
		*offset = update.ID + 1
	}
	return nil
}
