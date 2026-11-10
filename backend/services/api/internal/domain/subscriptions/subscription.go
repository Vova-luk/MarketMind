package subscriptions

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("subscription not found")

type Subscription struct {
	ID           int64
	BaseAsset    string
	QuoteAsset   string
	ExchangeName string
	Price        *string
}

type Detail struct {
	Subscription
	UpdatedAt time.Time
	Alerts    []Alert
}

type Alert struct {
	ID             int64
	Type           string
	Value          string
	Direction      string
	ReferencePrice string
	TargetPrice    string
	IsActive       bool
}

type Repository interface {
	List(ctx context.Context, telegramUserID int64, limit int, offset int64) ([]Subscription, error)
	Get(ctx context.Context, telegramUserID, subscriptionID int64) (Detail, error)
	Delete(ctx context.Context, telegramUserID, subscriptionID int64) error
}
