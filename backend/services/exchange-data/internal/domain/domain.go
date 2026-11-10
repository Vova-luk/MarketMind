package domain

import "time"

type Market struct {
	Exchange       string
	Cryptocurrency []Cryptocurrency
	ReceivedAt     time.Time
}

type Cryptocurrency struct {
	Symbol string
	Price  string
	Error  string
}

type Instrument struct {
	ExternalID string
	BaseAsset  string
	QuoteAsset string
}
