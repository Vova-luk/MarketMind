package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/sync/semaphore"

	"marketmind/contracts/events"
)

const (
	initialRetryDelay = time.Second
	maxRetryDelay     = time.Minute
	retryLogMessage   = "Telegram delivery failed, retrying"
)

type Sender interface {
	SendMessage(ctx context.Context, userID int64, text string) error
}

type retryAfterError interface {
	RetryAfter() time.Duration
}

type delivery struct {
	notificationID string
	userID         int64
	text           string
	retryDelay     time.Duration
	retryAt        time.Time
}

type Service struct {
	sender Sender
	slots  *semaphore.Weighted
	logger *slog.Logger
}

func NewService(sender Sender, workers int, logger *slog.Logger) *Service {
	return &Service{sender: sender, slots: semaphore.NewWeighted(int64(workers)), logger: logger}
}

// HandlePriceChanged tries each valid notification before retrying failed sends.
// An invalid notification prevents the Kafka record from being acknowledged.
func (s *Service) HandlePriceChanged(ctx context.Context, event events.PriceChangedEvent) error {
	if event.EventID == "" || len(event.Items) == 0 {
		return fmt.Errorf("event ID and notifications are required")
	}
	var invalid []error
	var pending []delivery
	for _, item := range event.Items {
		text, err := notificationText(item)
		if err != nil {
			invalid = append(invalid, fmt.Errorf("notification %q: %w", item.NotificationID, err))
			continue
		}
		message := delivery{
			notificationID: item.NotificationID,
			userID:         item.TelegramUserID,
			text:           text,
		}
		if err := s.attemptDelivery(ctx, &message); err != nil {
			return err
		}
		if !message.retryAt.IsZero() {
			pending = append(pending, message)
		}
	}
	if err := s.retryDeliveries(ctx, pending); err != nil {
		return err
	}
	return errors.Join(invalid...)
}

func (s *Service) attemptDelivery(ctx context.Context, message *delivery) error {
	if err := s.slots.Acquire(ctx, 1); err != nil {
		return err
	}
	err := s.sender.SendMessage(ctx, message.userID, message.text)
	s.slots.Release(1)
	if err == nil {
		message.retryAt = time.Time{}
		return nil
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	message.retryDelay = min(max(initialRetryDelay, 2*message.retryDelay), maxRetryDelay)
	wait := message.retryDelay
	var retry retryAfterError
	if errors.As(err, &retry) {
		wait = max(wait, retry.RetryAfter())
	}
	message.retryAt = time.Now().Add(wait)
	s.logger.WarnContext(ctx, retryLogMessage,
		"notification_id", message.notificationID, "retry_in", wait, "error", err)
	return nil
}

func (s *Service) retryDeliveries(ctx context.Context, pending []delivery) error {
	for len(pending) > 0 {
		var next time.Time
		remaining := pending[:0]
		for _, message := range pending {
			if !time.Now().Before(message.retryAt) {
				if err := s.attemptDelivery(ctx, &message); err != nil {
					return err
				}
			}
			if message.retryAt.IsZero() {
				continue
			}
			remaining = append(remaining, message)
			if next.IsZero() || message.retryAt.Before(next) {
				next = message.retryAt
			}
		}
		pending = remaining
		if next.IsZero() {
			break
		}
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return nil
}
