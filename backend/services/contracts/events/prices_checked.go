package events

import "time"

// price-checked-topic
type PricesCheckedEvent struct {
	EventID string         `json:"event_id"`
	Items   []CheckedPrice `json:"items"`
}

type CheckedPrice struct {
	ExchangeName string    `json:"exchange_name"`
	MarketID     string    `json:"market_id"`
	ExternalID   string    `json:"external_id"`
	Price        string    `json:"price"`
	ReceivedAt   time.Time `json:"received_at"`
}

// price-changed-topic
type PriceChangedEvent struct {
	EventID string              `json:"event_id"`
	Items   []PriceNotification `json:"items"`
}

type PriceNotification struct {
	NotificationID string `json:"notification_id"`
	TelegramUserID int64  `json:"telegram_user_id"`
	AlertID        int64  `json:"alert_id"`
	MarketID       string `json:"market_id"`

	ExchangeName string `json:"exchange_name"`
	BaseAsset    string `json:"base_asset"`
	QouteAsset   string `json:"qoute_asset"`

	Type           string  `json:"type"`
	Value          float64 `json:"value"`
	CurrentPrice   string  `json:"current_price"`
	Direction      string  `json:"direction"`
	ReferencePrice string  `json:"reference_price"`
}
