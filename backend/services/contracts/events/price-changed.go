package events

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
	QuoteAsset   string `json:"quote_asset"`

	Type           string `json:"type"`
	Value          string `json:"value"`
	CurrentPrice   string `json:"current_price"`
	Direction      string `json:"direction"`
	ReferencePrice string `json:"reference_price"`
}
