package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"marketmind/contracts/events"
	"marketmind/tracking/internal/domain"
)

type Repository interface {
	GetMarket(ctx context.Context, marketID int64) (domain.Market, error)
	GetActiveAlerts(ctx context.Context, marketID int64) ([]domain.Alert, error)
	SetAlertNotified(ctx context.Context, alertID int64, notified bool) error
	UpdateMarketPrice(ctx context.Context, price events.CheckedPrice) error
}

type PriceChangedProducer interface {
	SendPriceChangedEvent(ctx context.Context, event events.PriceChangedEvent) error
}

type Service struct {
	repository Repository
	producer   PriceChangedProducer
}

func NewService(producer PriceChangedProducer, repository Repository) *Service {
	return &Service{repository: repository, producer: producer}
}

func (s *Service) HandlePricesChecked(ctx context.Context, event events.PricesCheckedEvent) error {
	if strings.TrimSpace(event.EventID) == "" {
		return fmt.Errorf("event ID is required")
	}
	for _, price := range event.Items {
		if err := s.checkMarket(ctx, event.EventID, price); err != nil {
			return fmt.Errorf("check market %s: %w", price.MarketID, err)
		}
	}
	return nil
}

func (s *Service) checkMarket(ctx context.Context, eventID string, quote events.CheckedPrice) error {
	id, err := strconv.ParseInt(quote.MarketID, 10, 64)
	if err != nil || id <= 0 {
		return fmt.Errorf("invalid market ID %q", quote.MarketID)
	}
	current, err := positiveDecimal(quote.Price)
	if err != nil {
		return err
	}

	if quote.ReceivedAt.IsZero() {
		return fmt.Errorf("quote received time is required")
	}

	market, err := s.repository.GetMarket(ctx, id)
	if err != nil {
		return fmt.Errorf("get market: %w", err)
	}
	if market.ID != id || !strings.EqualFold(market.ExchangeName, quote.ExchangeName) || market.ExternalID != quote.ExternalID {
		return fmt.Errorf("quote does not match stored market")
	}
	if !quote.ReceivedAt.After(market.UpdatedAt) {
		return nil
	}
	previous, err := positiveDecimal(market.Price)
	if err != nil {
		return fmt.Errorf("stored price: %w", err)
	}
	alerts, err := s.repository.GetActiveAlerts(ctx, id)
	if err != nil {
		return fmt.Errorf("get alerts: %w", err)
	}

	output := events.PriceChangedEvent{EventID: fmt.Sprintf("%s:%d", eventID, id)}
	var notifiedIDs []int64
	for _, alert := range alerts {
		target, err := threshold(alert)
		if err != nil {
			return fmt.Errorf("alert %d: %w", alert.AlertID, err)
		}
		matched, err := matches(current, target, alert.Direction)
		if err != nil {
			return err
		}
		if !matched {
			if alert.IsNotified {
				if err := s.repository.SetAlertNotified(ctx, alert.AlertID, false); err != nil {
					return fmt.Errorf("rearm alert %d: %w", alert.AlertID, err)
				}
			}
			continue
		}
		wasMatched, _ := matches(previous, target, alert.Direction)
		if alert.IsNotified || wasMatched || !market.ExchangeActive || !alert.NotificationEnabled {
			continue
		}
		output.Items = append(output.Items, events.PriceNotification{
			NotificationID: fmt.Sprintf("%s:%d", eventID, alert.AlertID),
			TelegramUserID: alert.TelegramUserID, AlertID: alert.AlertID,
			MarketID: quote.MarketID, ExchangeName: market.ExchangeName,
			BaseAsset: market.BaseAsset, QuoteAsset: market.QuoteAsset,
			Type: alert.Type, Value: alert.Value, CurrentPrice: quote.Price,
			Direction: alert.Direction, ReferencePrice: alert.ReferencePrice,
		})
		notifiedIDs = append(notifiedIDs, alert.AlertID)
	}

	if len(output.Items) > 0 {
		if err := s.producer.SendPriceChangedEvent(ctx, output); err != nil {
			return fmt.Errorf("publish notifications: %w", err)
		}
		for _, id := range notifiedIDs {
			if err := s.repository.SetAlertNotified(ctx, id, true); err != nil {
				return fmt.Errorf("mark alert %d notified: %w", id, err)
			}
		}
	}
	if err := s.repository.UpdateMarketPrice(ctx, quote); err != nil {
		return fmt.Errorf("update market price: %w", err)
	}
	return nil
}
