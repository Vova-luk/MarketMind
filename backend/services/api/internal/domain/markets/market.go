package markets

type Instrument struct {
	ExternalID string
	BaseAsset  string
	QuoteAsset string
	Price      *string
}

type TrackedMarket struct {
	ExchangeName string
	MarketID     int64
	ExternalID   string
}
