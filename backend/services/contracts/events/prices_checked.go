package events

import "time"

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
