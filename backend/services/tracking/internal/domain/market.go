package domain

import "time"

type Market struct {
	ID             int64
	ExchangeName   string
	ExchangeActive bool
	ExternalID     string
	BaseAsset      string
	QuoteAsset     string
	Price          string
	UpdatedAt      time.Time
}
